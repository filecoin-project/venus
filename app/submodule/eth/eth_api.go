package eth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	"github.com/filecoin-project/go-state-types/builtin/v10/evm"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/ethhashlookup"
	"github.com/filecoin-project/venus/pkg/events"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/messagepool"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/venus-shared/actors"
	builtinactors "github.com/filecoin-project/venus/venus-shared/actors/builtin"
	builtinevm "github.com/filecoin-project/venus/venus-shared/actors/builtin/evm"
	types2 "github.com/filecoin-project/venus/venus-shared/actors/types"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var log = logging.Logger("eth_api")

func newEthAPI(em *EthSubModule) (*ethAPI, error) {
	a := &ethAPI{
		em:    em,
		chain: em.chainModule.API(),
		mpool: em.mpoolModule.API(),
	}

	dbPath := filepath.Join(a.em.sqlitePath, "txhash.db")

	// Check if the db exists, if not, we'll back-fill some entries
	_, err := os.Stat(dbPath)
	dbAlreadyExists := err == nil

	transactionHashLookup, err := ethhashlookup.NewTransactionHashLookup(dbPath)
	if err != nil {
		return nil, err
	}

	a.ethTxHashManager = &ethTxHashManager{
		chainAPI:              a.chain,
		messageStore:          em.chainModule.MessageStore,
		forkUpgradeConfig:     em.cfg.NetworkParams.ForkUpgradeParam,
		TransactionHashLookup: transactionHashLookup,
	}

	if !dbAlreadyExists {
		err = a.ethTxHashManager.PopulateExistingMappings(em.ctx, 0)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

type ethAPI struct {
	em               *EthSubModule
	chain            v1.IChain
	mpool            v1.IMessagePool
	ethTxHashManager *ethTxHashManager
}

func (a *ethAPI) start(ctx context.Context) error {
	const ChainHeadConfidence = 1
	ev, err := events.NewEventsWithConfidence(ctx, a.chain, ChainHeadConfidence)
	if err != nil {
		return err
	}

	// Tipset listener
	_ = ev.Observe(a.ethTxHashManager)

	ch, err := a.em.mpoolModule.MPool.Updates(ctx)
	if err != nil {
		return err
	}
	go waitForMpoolUpdates(ctx, ch, a.ethTxHashManager)
	go ethTxHashGC(ctx, a.em.cfg.FevmConfig.EthTxHashMappingLifetimeDays, a.ethTxHashManager)

	return nil
}

func (a *ethAPI) close() error {
	return a.ethTxHashManager.TransactionHashLookup.Close()
}

func (a *ethAPI) StateNetworkName(ctx context.Context) (types.NetworkName, error) {
	return a.chain.StateNetworkName(ctx)
}

func (a *ethAPI) EthBlockNumber(ctx context.Context) (types.EthUint64, error) {
	// eth_blockNumber needs to return the height of the latest committed tipset.
	// Ethereum clients expect all transactions included in this block to have execution outputs.
	// This is the parent of the head tipset. The head tipset is speculative, has not been
	// recognized by the network, and its messages are only included, not executed.
	// See https://github.com/filecoin-project/ref-fvm/issues/1135.
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthUint64(0), err
	}
	if height := head.Height(); height == 0 {
		// we're at genesis.
		return types.EthUint64(height), nil
	}
	// First non-null parent.
	effectiveParent := head.Parents()
	parent, err := a.chain.ChainGetTipSet(ctx, effectiveParent)
	if err != nil {
		return 0, err
	}
	return types.EthUint64(parent.Height()), nil
}

func (a *ethAPI) EthAccounts(context.Context) ([]types.EthAddress, error) {
	// The lotus node is not expected to hold manage accounts, so we'll always return an empty array
	return []types.EthAddress{}, nil
}

func (a *ethAPI) EthAddressToFilecoinAddress(ctx context.Context, ethAddress types.EthAddress) (address.Address, error) {
	return ethAddress.ToFilecoinAddress()
}

func (a *ethAPI) FilecoinAddressToEthAddress(ctx context.Context, filecoinAddress address.Address) (types.EthAddress, error) {
	return types.EthAddressFromFilecoinAddress(filecoinAddress)
}

func (a *ethAPI) countTipsetMsgs(ctx context.Context, ts *types.TipSet) (int, error) {
	msgs, err := a.em.chainModule.MessageStore.MessagesForTipset(ts)
	if err != nil {
		return 0, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	return len(msgs), nil
}

func (a *ethAPI) EthGetBlockTransactionCountByNumber(ctx context.Context, blkNum types.EthUint64) (types.EthUint64, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(blkNum), false)
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}

	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthUint64(count), err
}

func (a *ethAPI) EthGetBlockTransactionCountByHash(ctx context.Context, blkHash types.EthHash) (types.EthUint64, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	count, err := a.countTipsetMsgs(ctx, ts)
	return types.EthUint64(count), err
}

func (a *ethAPI) EthGetBlockByHash(ctx context.Context, blkHash types.EthHash, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := a.em.chainModule.ChainReader.GetTipSetByCid(ctx, blkHash.ToCid())
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading tipset %s: %w", ts, err)
	}
	return newEthBlockFromFilecoinTipSet(ctx, ts, fullTxInfo, a.em.chainModule.MessageStore, a.chain)
}

func (a *ethAPI) parseBlkParam(ctx context.Context, blkParam string) (tipset *types.TipSet, err error) {
	if blkParam == "earliest" {
		return nil, fmt.Errorf("block param \"earliest\" is not supported")
	}

	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}
	switch blkParam {
	case "pending":
		return head, nil
	case "latest":
		parent, err := a.chain.ChainGetTipSet(ctx, head.Parents())
		if err != nil {
			return nil, fmt.Errorf("cannot get parent tipset")
		}
		return parent, nil
	default:
		var num types.EthUint64
		err := num.UnmarshalJSON([]byte(`"` + blkParam + `"`))
		if err != nil {
			return nil, fmt.Errorf("cannot parse block number: %v", err)
		}
		ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(num), false)
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", num)
		}
		return ts, nil
	}
}

func (a *ethAPI) EthGetBlockByNumber(ctx context.Context, blkParam string, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return types.EthBlock{}, err
	}
	return newEthBlockFromFilecoinTipSet(ctx, ts, fullTxInfo, a.em.chainModule.MessageStore, a.chain)
}

func (a *ethAPI) EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error) {
	// Ethereum's behavior is to return null when the txHash is invalid, so we use nil to check if txHash is valid
	if txHash == nil {
		return nil, nil
	}

	c, err := a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(*txHash)
	if err != nil {
		log.Debug("could not find transaction hash %s in lookup table", txHash.String())
	}

	// This isn't an eth transaction we have the mapping for, so let's look it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	// first, try to get the cid from mined transactions
	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, constants.LookbackNoLimit, true)
	if err == nil && msgLookup != nil {
		tx, err := newEthTxFromMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.chain)
		if err == nil {
			return &tx, nil
		}
	}

	// if not found, try to get it from the mempool
	pending, err := a.mpool.MpoolPending(ctx, types.EmptyTSK)
	if err != nil {
		// inability to fetch mpool pending transactions is an internal node error
		// that needs to be reported as-is
		return nil, fmt.Errorf("cannot get pending txs from mpool: %s", err)
	}

	for _, p := range pending {
		if p.Cid() == c {
			tx, err := newEthTxFromSignedMessage(ctx, p, a.chain)
			if err != nil {
				return nil, fmt.Errorf("could not convert Filecoin message into tx: %v", err)
			}
			return &tx, nil
		}
	}
	// Ethereum clients expect an empty response when the message was not found
	return nil, nil
}

func (a *ethAPI) EthGetMessageCidByTransactionHash(ctx context.Context, txHash *types.EthHash) (*cid.Cid, error) {
	// Ethereum's behavior is to return null when the txHash is invalid, so we use nil to check if txHash is valid
	if txHash == nil {
		return nil, nil
	}

	c, err := a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(*txHash)
	// We fall out of the first condition and continue
	if errors.Is(err, ethhashlookup.ErrNotFound) {
		log.Debug("could not find transaction hash %s in lookup table", txHash.String())
	} else if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	} else {
		return &c, nil
	}

	// This isn't an eth transaction we have the mapping for, so let's try looking it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	_, err = a.em.chainModule.MessageStore.LoadSignedMessage(ctx, c)
	if err == nil {
		// This is an Eth Tx, Secp message, Or BLS message in the mpool
		return &c, nil
	}

	_, err = a.em.chainModule.MessageStore.LoadUnsignedMessage(ctx, c)
	if err == nil {
		// This is a BLS message
		return &c, nil
	}

	// Ethereum clients expect an empty response when the message was not found
	return nil, nil
}

func (a *ethAPI) EthGetTransactionHashByCid(ctx context.Context, cid cid.Cid) (*types.EthHash, error) {
	hash, err := ethTxHashFromMessageCid(ctx, cid, a.em.chainModule.MessageStore, a.chain)
	if hash == types.EmptyEthHash {
		// not found
		return nil, nil
	}

	return &hash, err
}

func (a *ethAPI) EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkParam string) (types.EthUint64, error) {
	addr, err := sender.ToFilecoinAddress()
	if err != nil {
		return types.EthUint64(0), err
	}
	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	// First, handle the case where the "sender" is an EVM actor.
	if actor, err := a.em.chainModule.Stmgr.GetActorAt(ctx, addr, ts); err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to lookup contract %s: %w", sender, err)
	} else if builtinactors.IsEvmActor(actor.Code) {
		evmState, err := builtinevm.Load(a.em.chainModule.ChainReader.Store(ctx), actor)
		if err != nil {
			return 0, fmt.Errorf("failed to load evm state: %w", err)
		}
		if alive, err := evmState.IsAlive(); err != nil {
			return 0, err
		} else if !alive {
			return 0, nil
		}
		nonce, err := evmState.Nonce()
		return types.EthUint64(nonce), err
	}

	nonce, err := a.em.mpoolModule.MPool.GetNonce(ctx, addr, ts.Key())
	if err != nil {
		return types.EthUint64(0), err
	}
	return types.EthUint64(nonce), nil
}

func (a *ethAPI) EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error) {
	c, err := a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(txHash)
	if err != nil {
		log.Debug("could not find transaction hash %s in lookup table", txHash.String())
	}

	// This isn't an eth transaction we have the mapping for, so let's look it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, constants.LookbackNoLimit, true)
	if err != nil || msgLookup == nil {
		return nil, nil
	}

	tx, err := newEthTxFromMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.chain)
	if err != nil {
		return nil, nil
	}

	replay, err := a.chain.StateReplay(ctx, types.EmptyTSK, c)
	if err != nil {
		return nil, nil
	}

	var events []types.Event
	if rct := replay.MsgRct; rct != nil && rct.EventsRoot != nil {
		events, err = a.chain.ChainGetEvents(ctx, *rct.EventsRoot)
		if err != nil {
			return nil, nil
		}
	}

	receipt, err := newEthTxReceipt(ctx, tx, msgLookup, replay, events, a.chain)
	if err != nil {
		return nil, nil
	}

	return &receipt, nil
}

func (a *ethAPI) EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, nil
}

func (a *ethAPI) EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, nil
}

// EthGetCode returns string value of the compiled bytecode
func (a *ethAPI) EthGetCode(ctx context.Context, ethAddr types.EthAddress, blkParam string) (types.EthBytes, error) {
	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return nil, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	// StateManager.Call will panic if there is no parent
	if ts.Height() == 0 {
		return nil, fmt.Errorf("block param must not specify genesis block")
	}

	actor, err := a.em.chainModule.Stmgr.GetActorAt(ctx, to, ts)
	if err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lookup contract %s: %w", ethAddr, err)
	}

	// Not a contract. We could try to distinguish between accounts and "native" contracts here,
	// but it's not worth it.
	if !builtinactors.IsEvmActor(actor.Code) {
		return nil, nil
	}

	msg := &types.Message{
		From:       builtinactors.SystemActorAddr,
		To:         to,
		Value:      big.Zero(),
		Method:     builtintypes.MethodsEVM.GetBytecode,
		Params:     nil,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}

	// Try calling until we find a height with no migration.
	var res *types.InvocResult
	for {
		res, err = a.em.chainModule.Stmgr.Call(ctx, msg, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to call GetBytecode: %w", err)
	}

	if res.MsgRct == nil {
		return nil, fmt.Errorf("no message receipt")
	}

	if res.MsgRct.ExitCode.IsError() {
		return nil, fmt.Errorf("GetBytecode failed: %s", res.Error)
	}

	var getBytecodeReturn evm.GetBytecodeReturn
	if err := getBytecodeReturn.UnmarshalCBOR(bytes.NewReader(res.MsgRct.Return)); err != nil {
		return nil, fmt.Errorf("failed to decode EVM bytecode CID: %w", err)
	}

	// The contract has selfdestructed, so the code is "empty".
	if getBytecodeReturn.Cid == nil {
		return nil, nil
	}

	blk, err := a.em.chainModule.ChainReader.Blockstore().Get(ctx, *getBytecodeReturn.Cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM bytecode: %w", err)
	}

	return blk.RawData(), nil
}

func (a *ethAPI) EthGetStorageAt(ctx context.Context, ethAddr types.EthAddress, position types.EthBytes, blkParam string) (types.EthBytes, error) {
	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return nil, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	l := len(position)
	if l > 32 {
		return nil, fmt.Errorf("supplied storage key is too long")
	}

	// pad with zero bytes if smaller than 32 bytes
	position = append(make([]byte, 32-l, 32), position...)

	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	// use the system actor as the caller
	from, err := address.NewIDAddress(0)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system sender address: %w", err)
	}

	actor, err := a.em.chainModule.Stmgr.GetActorAt(ctx, to, ts)
	if err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return types.EthBytes(make([]byte, 32)), nil
		}
		return nil, fmt.Errorf("failed to lookup contract %s: %w", ethAddr, err)
	}

	if !builtinactors.IsEvmActor(actor.Code) {
		return types.EthBytes(make([]byte, 32)), nil
	}

	params, err := actors.SerializeParams(&evm.GetStorageAtParams{
		StorageKey: *(*[32]byte)(position),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize parameters: %w", err)
	}

	msg := &types.Message{
		From:       from,
		To:         to,
		Value:      big.Zero(),
		Method:     builtintypes.MethodsEVM.GetStorageAt,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}

	// Try calling until we find a height with no migration.
	var res *types.InvocResult
	for {
		res, err = a.em.chainModule.Stmgr.Call(ctx, msg, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("call failed: %w", err)
	}

	if res.MsgRct == nil {
		return nil, fmt.Errorf("no message receipt")
	}

	if res.MsgRct.ExitCode.IsError() {
		return nil, fmt.Errorf("failed to lookup storage slot: %s", res.Error)
	}

	var ret abi.CborBytes
	if err := ret.UnmarshalCBOR(bytes.NewReader(res.MsgRct.Return)); err != nil {
		return nil, fmt.Errorf("failed to unmarshal storage slot: %w", err)
	}

	// pad with zero bytes if smaller than 32 bytes
	ret = append(make([]byte, 32-len(ret), 32), ret...)

	return types.EthBytes(ret), nil
}

func (a *ethAPI) EthGetBalance(ctx context.Context, address types.EthAddress, blkParam string) (types.EthBigInt, error) {
	filAddr, err := address.ToFilecoinAddress()
	if err != nil {
		return types.EthBigInt{}, err
	}

	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return types.EthBigInt{}, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	actor, err := a.chain.StateGetActor(ctx, filAddr, ts.Key())
	if err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return types.EthBigIntZero, nil
		}
		return types.EthBigInt{}, err
	}

	return types.EthBigInt{Int: actor.Balance.Int}, nil
}

func (a *ethAPI) EthChainId(ctx context.Context) (types.EthUint64, error) {
	return types.EthUint64(types2.Eip155ChainID), nil
}

func (a *ethAPI) EthFeeHistory(ctx context.Context, p jsonrpc.RawParams) (types.EthFeeHistory, error) {
	params, err := jsonrpc.DecodeParams[types.EthFeeHistoryParams](p)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("decoding params: %w", err)
	}
	if params.BlkCount > 1024 {
		return types.EthFeeHistory{}, fmt.Errorf("block count should be smaller than 1024")
	}

	rewardPercentiles := make([]float64, 0)
	if params.RewardPercentiles != nil {
		rewardPercentiles = append(rewardPercentiles, *params.RewardPercentiles...)
	}
	for i, rp := range rewardPercentiles {
		if rp < 0 || rp > 100 {
			return types.EthFeeHistory{}, fmt.Errorf("invalid reward percentile: %f should be between 0 and 100", rp)
		}
		if i > 0 && rp < rewardPercentiles[i-1] {
			return types.EthFeeHistory{}, fmt.Errorf("invalid reward percentile: %f should be larger than %f", rp, rewardPercentiles[i-1])
		}
	}

	ts, err := a.parseBlkParam(ctx, params.NewestBlkNum)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("bad block parameter %s: %s", params.NewestBlkNum, err)
	}

	// Deal with the case that the chain is shorter than the number of requested blocks.
	oldestBlkHeight := uint64(1)
	if abi.ChainEpoch(params.BlkCount) <= ts.Height() {
		oldestBlkHeight = uint64(ts.Height()) - uint64(params.BlkCount) + 1
	}

	// NOTE: baseFeePerGas should include the next block after the newest of the returned range,
	//  because the next base fee can be inferred from the messages in the newest block.
	//  However, this is NOT the case in Filecoin due to deferred execution, so the best
	//  we can do is duplicate the last value.
	baseFeeArray := []types.EthBigInt{types.EthBigInt(ts.Blocks()[0].ParentBaseFee)}
	gasUsedRatioArray := []float64{}
	rewardsArray := make([][]types.EthBigInt, 0)

	for ts.Height() >= abi.ChainEpoch(oldestBlkHeight) {
		// Unfortunately we need to rebuild the full message view so we can
		// totalize gas used in the tipset.
		msgs, err := a.em.chainModule.MessageStore.MessagesForTipset(ts)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
		}

		txGasRewards := gasRewardSorter{}
		for txIdx, msg := range msgs {
			msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, msg.Cid(), constants.LookbackNoLimit, false)
			if err != nil || msgLookup == nil {
				return types.EthFeeHistory{}, nil
			}

			tx, err := newEthTxFromMessageLookup(ctx, msgLookup, txIdx, a.em.chainModule.MessageStore, a.chain)
			if err != nil {
				return types.EthFeeHistory{}, nil
			}

			txGasRewards = append(txGasRewards, gasRewardTuple{
				reward: tx.Reward(ts.Blocks()[0].ParentBaseFee),
				gas:    uint64(msgLookup.Receipt.GasUsed),
			})
		}

		rewards, totalGasUsed := calculateRewardsAndGasUsed(rewardPercentiles, txGasRewards)

		// arrays should be reversed at the end
		baseFeeArray = append(baseFeeArray, types.EthBigInt(ts.Blocks()[0].ParentBaseFee))
		gasUsedRatioArray = append(gasUsedRatioArray, float64(totalGasUsed)/float64(constants.BlockGasLimit))
		rewardsArray = append(rewardsArray, rewards)

		parentTSKey := ts.Parents()
		ts, err = a.chain.ChainGetTipSet(ctx, parentTSKey)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("cannot load tipset key: %v", parentTSKey)
		}
	}

	// Reverse the arrays; we collected them newest to oldest; the client expects oldest to newest.
	for i, j := 0, len(baseFeeArray)-1; i < j; i, j = i+1, j-1 {
		baseFeeArray[i], baseFeeArray[j] = baseFeeArray[j], baseFeeArray[i]
	}
	for i, j := 0, len(gasUsedRatioArray)-1; i < j; i, j = i+1, j-1 {
		gasUsedRatioArray[i], gasUsedRatioArray[j] = gasUsedRatioArray[j], gasUsedRatioArray[i]
	}
	for i, j := 0, len(rewardsArray)-1; i < j; i, j = i+1, j-1 {
		rewardsArray[i], rewardsArray[j] = rewardsArray[j], rewardsArray[i]
	}

	ret := types.EthFeeHistory{
		OldestBlock:   types.EthUint64(oldestBlkHeight),
		BaseFeePerGas: baseFeeArray,
		GasUsedRatio:  gasUsedRatioArray,
	}
	if params.RewardPercentiles != nil {
		ret.Reward = &rewardsArray
	}

	return ret, nil
}

func (a *ethAPI) NetVersion(ctx context.Context) (string, error) {
	// Note that networkId is not encoded in hex
	nv, err := a.chain.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(uint64(nv), 10), nil
}

func (a *ethAPI) NetListening(ctx context.Context) (bool, error) {
	return true, nil
}

func (a *ethAPI) EthProtocolVersion(ctx context.Context) (types.EthUint64, error) {
	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthUint64(0), err
	}

	return types.EthUint64(a.em.chainModule.Fork.GetNetworkVersion(ctx, head.Height())), nil
}

func (a *ethAPI) EthMaxPriorityFeePerGas(ctx context.Context) (types.EthBigInt, error) {
	gasPremium, err := a.mpool.GasEstimateGasPremium(ctx, 0, builtin.SystemActorAddr, 10000, types.EmptyTSK)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}
	return types.EthBigInt(gasPremium), nil
}

func (a *ethAPI) EthGasPrice(ctx context.Context) (types.EthBigInt, error) {
	// According to Geth's implementation, eth_gasPrice should return base + tip
	// Ref: https://github.com/ethereum/pm/issues/328#issuecomment-853234014

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}
	baseFee := ts.Blocks()[0].ParentBaseFee

	premium, err := a.EthMaxPriorityFeePerGas(ctx)
	if err != nil {
		return types.EthBigInt(big.Zero()), err
	}

	gasPrice := big.Add(baseFee, big.Int(premium))
	return types.EthBigInt(gasPrice), nil
}

func (a *ethAPI) EthSendRawTransaction(ctx context.Context, rawTx types.EthBytes) (types.EthHash, error) {
	txArgs, err := types.ParseEthTxArgs(rawTx)
	if err != nil {
		return types.EmptyEthHash, err
	}

	smsg, err := txArgs.ToSignedMessage()
	if err != nil {
		return types.EmptyEthHash, err
	}

	_, err = a.mpool.MpoolPush(ctx, smsg)
	if err != nil {
		return types.EmptyEthHash, err
	}
	return types.EthHashFromTxBytes(rawTx), nil
}

func (a *ethAPI) ethCallToFilecoinMessage(ctx context.Context, tx types.EthCall) (*types.Message, error) {
	var from address.Address
	if tx.From == nil || *tx.From == (types.EthAddress{}) {
		var err error
		from, err = (types.EthAddress{}).ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to construct the ethereum system address: %w", err)
		}
	} else {
		// The from address must be translatable to an f4 address.
		var err error
		from, err = tx.From.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to translate sender address (%s): %w", tx.From.String(), err)
		}
		if p := from.Protocol(); p != address.Delegated {
			return nil, fmt.Errorf("expected a class 4 address, got: %d: %w", p, err)
		}
	}

	var params []byte
	if len(tx.Data) > 0 {
		initcode := abi.CborBytes(tx.Data)
		params2, err := actors.SerializeParams(&initcode)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize params: %w", err)
		}
		params = params2
	}

	var to address.Address
	var method abi.MethodNum
	if tx.To == nil {
		// this is a contract creation
		to = builtintypes.EthereumAddressManagerActorAddr
		method = builtintypes.MethodsEAM.CreateExternal
	} else {
		addr, err := tx.To.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
		}
		to = addr

		method = builtintypes.MethodsEVM.InvokeContract
	}

	return &types.Message{
		From:       from,
		To:         to,
		Value:      big.Int(tx.Value),
		Method:     method,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}, nil
}

func (a *ethAPI) applyMessage(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error) {
	ts, err := a.chain.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("failed to got tipset %v", err)
	}

	// Try calling until we find a height with no migration.
	var res *types.InvocResult
	for {
		res, err = a.em.chainModule.Stmgr.CallWithGas(ctx, msg, []types.ChainMsg{}, ts)
		if err != fork.ErrExpensiveFork {
			break
		}
		ts, err = a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("getting parent tipset: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("CallWithGas failed: %w", err)
	}
	if res.MsgRct == nil {
		return nil, fmt.Errorf("no message receipt")
	}
	if res.MsgRct.ExitCode.IsError() {
		reason := parseEthRevert(res.MsgRct.Return)
		return nil, fmt.Errorf("message execution failed: exit %s, revert reason: %s, vm error: %s", res.MsgRct.ExitCode, reason, res.Error)
	}

	return res, nil
}

func (a *ethAPI) EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error) {
	msg, err := a.ethCallToFilecoinMessage(ctx, tx)
	if err != nil {
		return types.EthUint64(0), err
	}
	// Set the gas limit to the zero sentinel value, which makes
	// gas estimation actually run.
	msg.GasLimit = 0

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthUint64(0), err
	}
	gassedMsg, err := a.mpool.GasEstimateMessageGas(ctx, msg, nil, ts.Key())
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("failed to estimate gas: %w", err)
	}
	if err != nil {
		// On failure, GasEstimateMessageGas doesn't actually return the invocation result,
		// it just returns an error. That means we can't get the revert reason.
		//
		// So we re-execute the message with EthCall (well, applyMessage which contains the
		// guts of EthCall). This will give us an ethereum specific error with revert
		// information.
		msg.GasLimit = constants.BlockGasLimit
		if _, err2 := a.applyMessage(ctx, msg, ts.Key()); err2 != nil {
			err = err2
		}
		return types.EthUint64(0), fmt.Errorf("failed to estimate gas: %w", err)
	}

	expectedGas, err := ethGasSearch(ctx, a.em.chainModule.Stmgr, a.em.mpoolModule.MPool, gassedMsg, ts)
	if err != nil {
		return 0, fmt.Errorf("gas search failed: %w", err)
	}

	return types.EthUint64(expectedGas), nil
}

// gasSearch does an exponential search to find a gas value to execute the
// message with. It first finds a high gas limit that allows the message to execute
// by doubling the previous gas limit until it succeeds then does a binary
// search till it gets within a range of 1%
func gasSearch(
	ctx context.Context,
	smgr *statemanger.Stmgr,
	msgIn *types.Message,
	priorMsgs []types.ChainMsg,
	ts *types.TipSet,
) (int64, error) {
	msg := *msgIn

	high := msg.GasLimit
	low := msg.GasLimit

	canSucceed := func(limit int64) (bool, error) {
		msg.GasLimit = limit

		res, err := smgr.CallWithGas(ctx, &msg, priorMsgs, ts)
		if err != nil {
			return false, fmt.Errorf("CallWithGas failed: %w", err)
		}

		if res.MsgRct.ExitCode.IsSuccess() {
			return true, nil
		}

		return false, nil
	}

	for {
		ok, err := canSucceed(high)
		if err != nil {
			return -1, fmt.Errorf("searching for high gas limit failed: %w", err)
		}
		if ok {
			break
		}

		low = high
		high = high * 2

		if high > constants.BlockGasLimit {
			high = constants.BlockGasLimit
			break
		}
	}

	checkThreshold := high / 100
	for (high - low) > checkThreshold {
		median := (low + high) / 2
		ok, err := canSucceed(median)
		if err != nil {
			return -1, fmt.Errorf("searching for optimal gas limit failed: %w", err)
		}

		if ok {
			high = median
		} else {
			low = median
		}

		checkThreshold = median / 100
	}

	return high, nil
}

func traceContainsExitCode(et types.ExecutionTrace, ex exitcode.ExitCode) bool {
	if et.MsgRct.ExitCode == ex {
		return true
	}

	for _, et := range et.Subcalls {
		if traceContainsExitCode(et, ex) {
			return true
		}
	}

	return false
}

// ethGasSearch executes a message for gas estimation using the previously estimated gas.
// If the message fails due to an out of gas error then a gas search is performed.
// See gasSearch.
func ethGasSearch(
	ctx context.Context,
	stmgr *statemanger.Stmgr,
	mpool *messagepool.MessagePool,
	msgIn *types.Message,
	ts *types.TipSet,
) (int64, error) {
	msg := *msgIn
	currTS := ts

	res, priorMsgs, ts, err := mpool.GasEstimateCallWithGas(ctx, &msg, currTS)
	if err != nil {
		return -1, fmt.Errorf("gas estimation failed: %w", err)
	}

	if res.MsgRct.ExitCode.IsSuccess() {
		return msg.GasLimit, nil
	}
	if traceContainsExitCode(res.ExecutionTrace, exitcode.SysErrOutOfGas) {
		ret, err := gasSearch(ctx, stmgr, &msg, priorMsgs, ts)
		if err != nil {
			return -1, fmt.Errorf("gas estimation search failed: %w", err)
		}

		ret = int64(float64(ret) * mpool.GetConfig().GasLimitOverestimation)
		return ret, nil
	}

	return -1, fmt.Errorf("message execution failed: exit %s, reason: %s", res.MsgRct.ExitCode, res.Error)
}

func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam string) (types.EthBytes, error) {
	msg, err := a.ethCallToFilecoinMessage(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ethcall to filecoin message: %w", err)
	}
	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return nil, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	invokeResult, err := a.applyMessage(ctx, msg, ts.Key())
	if err != nil {
		return nil, err
	}

	if msg.To == builtintypes.EthereumAddressManagerActorAddr {
		// As far as I can tell, the Eth API always returns empty on contract deployment
		return types.EthBytes{}, nil
	} else if len(invokeResult.MsgRct.Return) > 0 {
		return cbg.ReadByteArray(bytes.NewReader(invokeResult.MsgRct.Return), uint64(len(invokeResult.MsgRct.Return)))
	}

	return types.EthBytes{}, nil
}

func (a *ethAPI) Web3ClientVersion(ctx context.Context) (string, error) {
	return constants.UserVersion(), nil
}

func newEthBlockFromFilecoinTipSet(ctx context.Context, ts *types.TipSet, fullTxInfo bool, ms *chain.MessageStore, ca v1.IChain) (types.EthBlock, error) {
	parent, err := ca.ChainGetTipSet(ctx, ts.Parents())
	if err != nil {
		return types.EthBlock{}, err
	}
	parentKeyCid, err := parent.Key().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	parentBlkHash, err := types.EthHashFromCid(parentKeyCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	blkCid, err := ts.Key().Cid()
	if err != nil {
		return types.EthBlock{}, err
	}
	blkHash, err := types.EthHashFromCid(blkCid)
	if err != nil {
		return types.EthBlock{}, err
	}

	msgs, err := ms.MessagesForTipset(ts)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("error loading messages for tipset: %v: %w", ts, err)
	}

	block := types.NewEthBlock(len(msgs) > 0)

	// this seems to be a very expensive way to get gasUsed of the block. may need to find an efficient way to do it
	gasUsed := int64(0)
	for txIdx, msg := range msgs {
		msgLookup, err := ca.StateSearchMsg(ctx, types.EmptyTSK, msg.Cid(), constants.LookbackNoLimit, false)
		if err != nil || msgLookup == nil {
			return types.EthBlock{}, nil
		}
		gasUsed += msgLookup.Receipt.GasUsed

		tx, err := newEthTxFromMessageLookup(ctx, msgLookup, txIdx, ms, ca)
		if err != nil {
			return types.EthBlock{}, nil
		}

		if fullTxInfo {
			block.Transactions = append(block.Transactions, tx)
		} else {
			block.Transactions = append(block.Transactions, tx.Hash.String())
		}
	}

	block.Hash = blkHash
	block.Number = types.EthUint64(ts.Height())
	block.ParentHash = parentBlkHash
	block.Timestamp = types.EthUint64(ts.Blocks()[0].Timestamp)
	block.BaseFeePerGas = types.EthBigInt{Int: ts.Blocks()[0].ParentBaseFee.Int}
	block.GasUsed = types.EthUint64(gasUsed)
	return block, nil
}

// lookupEthAddress makes its best effort at finding the Ethereum address for a
// Filecoin address. It does the following:
//
//  1. If the supplied address is an f410 address, we return its payload as the EthAddress.
//  2. Otherwise (f0, f1, f2, f3), we look up the actor on the state tree. If it has a delegated address, we return it if it's f410 address.
//  3. Otherwise, we fall back to returning a masked ID Ethereum address. If the supplied address is an f0 address, we
//     use that ID to form the masked ID address.
//  4. Otherwise, we fetch the actor's ID from the state tree and form the masked ID with it.
func lookupEthAddress(ctx context.Context, addr address.Address, ca v1.IChain) (types.EthAddress, error) {
	// BLOCK A: We are trying to get an actual Ethereum address from an f410 address.
	// Attempt to convert directly, if it's an f4 address.
	ethAddr, err := types.EthAddressFromFilecoinAddress(addr)
	if err == nil && !ethAddr.IsMaskedID() {
		return ethAddr, nil
	}

	// Lookup on the target actor and try to get an f410 address.
	actor, err := ca.StateGetActor(ctx, addr, types.EmptyTSK)
	if err != nil {
		return types.EthAddress{}, err
	}
	if actor.Address != nil {
		if ethAddr, err := types.EthAddressFromFilecoinAddress(*actor.Address); err == nil && !ethAddr.IsMaskedID() {
			return ethAddr, nil
		}
	}

	// BLOCK B: We gave up on getting an actual Ethereum address and are falling back to a Masked ID address.
	// Check if we already have an ID addr, and use it if possible.
	if err == nil && ethAddr.IsMaskedID() {
		return ethAddr, nil
	}

	// Otherwise, resolve the ID addr.
	idAddr, err := ca.StateLookupID(ctx, addr, types.EmptyTSK)
	if err != nil {
		return types.EthAddress{}, err
	}
	return types.EthAddressFromFilecoinAddress(idAddr)
}

func ethTxHashFromMessageCid(ctx context.Context, c cid.Cid, ms *chain.MessageStore, ca v1.IChain) (types.EthHash, error) {
	smsg, err := ms.LoadSignedMessage(ctx, c)
	if err == nil {
		// This is an Eth Tx, Secp message, Or BLS message in the mpool
		return ethTxHashFromSignedMessage(ctx, smsg, ca)
	}

	return types.EthHashFromCid(c)
}

func ethTxHashFromSignedMessage(ctx context.Context, smsg *types.SignedMessage, ca v1.IChain) (types.EthHash, error) {
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		ethTx, err := newEthTxFromSignedMessage(ctx, smsg, ca)
		if err != nil {
			return types.EmptyEthHash, err
		}
		return ethTx.Hash, nil
	} else if smsg.Signature.Type == crypto.SigTypeSecp256k1 {
		return types.EthHashFromCid(smsg.Cid())
	} else { // BLS message
		return types.EthHashFromCid(smsg.Message.Cid())
	}
}

func newEthTxFromSignedMessage(ctx context.Context, smsg *types.SignedMessage, ca v1.IChain) (types.EthTx, error) {
	var tx types.EthTx
	var err error

	// This is an eth tx
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		tx, err = types.EthTxFromSignedEthMessage(smsg)
		if err != nil {
			return types.EthTx{}, fmt.Errorf("failed to convert from signed message: %w", err)
		}

		tx.Hash, err = tx.TxHash()
		if err != nil {
			return types.EthTx{}, fmt.Errorf("failed to calculate hash for ethTx: %w", err)
		}

		fromAddr, err := lookupEthAddress(ctx, smsg.Message.From, ca)
		if err != nil {
			return types.EthTx{}, fmt.Errorf("failed to resolve Ethereum address: %w", err)
		}

		tx.From = fromAddr
	} else if smsg.Signature.Type == crypto.SigTypeSecp256k1 { // Secp Filecoin Message
		tx = ethTxFromNativeMessage(ctx, smsg.VMMessage(), ca)
		tx.Hash, err = types.EthHashFromCid(smsg.Cid())
		if err != nil {
			return tx, err
		}
	} else { // BLS Filecoin message
		tx = ethTxFromNativeMessage(ctx, smsg.VMMessage(), ca)
		tx.Hash, err = types.EthHashFromCid(smsg.Message.Cid())
		if err != nil {
			return tx, err
		}
	}

	return tx, nil
}

// ethTxFromNativeMessage does NOT populate:
// - BlockHash
// - BlockNumber
// - TransactionIndex
// - Hash
func ethTxFromNativeMessage(ctx context.Context, msg *types.Message, ca v1.IChain) types.EthTx {
	// We don't care if we error here, conversion is best effort for non-eth transactions
	from, _ := lookupEthAddress(ctx, msg.From, ca)
	to, _ := lookupEthAddress(ctx, msg.To, ca)
	return types.EthTx{
		To:                   &to,
		From:                 from,
		Nonce:                types.EthUint64(msg.Nonce),
		ChainID:              types.EthUint64(types2.Eip155ChainID),
		Value:                types.EthBigInt(msg.Value),
		Type:                 types.Eip1559TxType,
		Gas:                  types.EthUint64(msg.GasLimit),
		MaxFeePerGas:         types.EthBigInt(msg.GasFeeCap),
		MaxPriorityFeePerGas: types.EthBigInt(msg.GasPremium),
		AccessList:           []types.EthHash{},
	}
}

// newEthTxFromMessageLookup creates an ethereum transaction from filecoin message lookup. If a negative txIdx is passed
// into the function, it looks up the transaction index of the message in the tipset, otherwise it uses the txIdx passed into the
// function
func newEthTxFromMessageLookup(ctx context.Context, msgLookup *types.MsgLookup, txIdx int, ms *chain.MessageStore, ca v1.IChain) (types.EthTx, error) {
	if msgLookup == nil {
		return types.EthTx{}, fmt.Errorf("msg does not exist")
	}

	ts, err := ca.ChainGetTipSet(ctx, msgLookup.TipSet)
	if err != nil {
		return types.EthTx{}, err
	}

	// This tx is located in the parent tipset
	parentTS, err := ca.ChainGetTipSet(ctx, ts.Parents())
	if err != nil {
		return types.EthTx{}, err
	}

	parentTSCid, err := parentTS.Key().Cid()
	if err != nil {
		return types.EthTx{}, err
	}

	// lookup the transactionIndex
	if txIdx < 0 {
		msgs, err := ms.MessagesForTipset(parentTS)
		if err != nil {
			return types.EthTx{}, err
		}
		for i, msg := range msgs {
			if msg.Cid() == msgLookup.Message {
				txIdx = i
				break
			}
		}
		if txIdx < 0 {
			return types.EthTx{}, fmt.Errorf("cannot find the msg in the tipset")
		}
	}

	blkHash, err := types.EthHashFromCid(parentTSCid)
	if err != nil {
		return types.EthTx{}, err
	}

	smsg, err := ms.LoadSignedMessage(ctx, msgLookup.Message)
	if err != nil {
		// We couldn't find the signed message, it might be a BLS message, so search for a regular message.
		msg, err := ms.LoadUnsignedMessage(ctx, msgLookup.Message)
		if err != nil {
			return types.EthTx{}, err
		}
		smsg = &types.SignedMessage{
			Message: *msg,
			Signature: crypto.Signature{
				Type: crypto.SigTypeUnknown,
				Data: nil,
			},
		}
	}

	tx, err := newEthTxFromSignedMessage(ctx, smsg, ca)
	if err != nil {
		return types.EthTx{}, err
	}

	var (
		bn = types.EthUint64(parentTS.Height())
		ti = types.EthUint64(txIdx)
	)

	tx.ChainID = types.EthUint64(types2.Eip155ChainID)
	tx.BlockHash = &blkHash
	tx.BlockNumber = &bn
	tx.TransactionIndex = &ti
	return tx, nil
}

func newEthTxReceipt(ctx context.Context, tx types.EthTx, lookup *types.MsgLookup, replay *types.InvocResult, events []types.Event, ca v1.IChain) (types.EthTxReceipt, error) {
	var (
		transactionIndex types.EthUint64
		blockHash        types.EthHash
		blockNumber      types.EthUint64
	)

	if tx.TransactionIndex != nil {
		transactionIndex = *tx.TransactionIndex
	}
	if tx.BlockHash != nil {
		blockHash = *tx.BlockHash
	}
	if tx.BlockNumber != nil {
		blockNumber = *tx.BlockNumber
	}

	receipt := types.EthTxReceipt{
		TransactionHash:  tx.Hash,
		From:             tx.From,
		To:               tx.To,
		TransactionIndex: transactionIndex,
		BlockHash:        blockHash,
		BlockNumber:      blockNumber,
		Type:             types.EthUint64(2),
		Logs:             []types.EthLog{}, // empty log array is compulsory when no logs, or libraries like ethers.js break
		LogsBloom:        types.EmptyEthBloom[:],
	}

	if lookup.Receipt.ExitCode.IsSuccess() {
		receipt.Status = 1
	}
	if lookup.Receipt.ExitCode.IsError() {
		receipt.Status = 0
	}

	receipt.GasUsed = types.EthUint64(lookup.Receipt.GasUsed)

	// TODO: handle CumulativeGasUsed
	receipt.CumulativeGasUsed = types.EmptyEthInt

	effectiveGasPrice := big.Div(replay.GasCost.TotalCost, big.NewInt(lookup.Receipt.GasUsed))
	receipt.EffectiveGasPrice = types.EthBigInt(effectiveGasPrice)

	if receipt.To == nil && lookup.Receipt.ExitCode.IsSuccess() {
		// Create and Create2 return the same things.
		var ret eam.CreateExternalReturn
		if err := ret.UnmarshalCBOR(bytes.NewReader(lookup.Receipt.Return)); err != nil {
			return types.EthTxReceipt{}, fmt.Errorf("failed to parse contract creation result: %w", err)
		}
		addr := types.EthAddress(ret.EthAddress)
		receipt.ContractAddress = &addr
	}

	if len(events) > 0 {
		receipt.Logs = make([]types.EthLog, 0, len(events))
		for i, evt := range events {
			l := types.EthLog{
				Removed:          false,
				LogIndex:         types.EthUint64(i),
				TransactionHash:  tx.Hash,
				TransactionIndex: transactionIndex,
				BlockHash:        blockHash,
				BlockNumber:      blockNumber,
			}

			data, topics, ok := ethLogFromEvent(evt.Entries)
			if !ok {
				// not an eth event.
				continue
			}
			for _, topic := range topics {
				types.EthBloomSet(receipt.LogsBloom, topic[:])
			}
			l.Data = data
			l.Topics = topics

			addr, err := address.NewIDAddress(uint64(evt.Emitter))
			if err != nil {
				return types.EthTxReceipt{}, fmt.Errorf("failed to create ID address: %w", err)
			}

			l.Address, err = lookupEthAddress(ctx, addr, ca)
			if err != nil {
				return types.EthTxReceipt{}, fmt.Errorf("failed to resolve Ethereum address: %w", err)
			}

			types.EthBloomSet(receipt.LogsBloom, l.Address[:])
			receipt.Logs = append(receipt.Logs, l)
		}
	}

	return receipt, nil
}

type ethTxHashManager struct {
	chainAPI              v1.IChain
	messageStore          *chain.MessageStore
	forkUpgradeConfig     *config.ForkUpgradeConfig
	TransactionHashLookup *ethhashlookup.EthTxHashLookup
}

func (m *ethTxHashManager) Apply(ctx context.Context, from, to *types.TipSet) error {
	for _, blk := range to.Blocks() {
		blkMsgs, err := m.chainAPI.ChainGetBlockMessages(ctx, blk.Cid())
		if err != nil {
			return err
		}

		for _, smsg := range blkMsgs.SecpkMessages {
			if smsg.Signature.Type != crypto.SigTypeDelegated {
				continue
			}

			hash, err := ethTxHashFromSignedMessage(ctx, smsg, m.chainAPI)
			if err != nil {
				return err
			}

			err = m.TransactionHashLookup.UpsertHash(hash, smsg.Cid())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *ethTxHashManager) Revert(ctx context.Context, from, to *types.TipSet) error {
	return nil
}

func (m *ethTxHashManager) PopulateExistingMappings(ctx context.Context, minHeight abi.ChainEpoch) error {
	if minHeight < m.forkUpgradeConfig.UpgradeHyggeHeight {
		minHeight = m.forkUpgradeConfig.UpgradeHyggeHeight
	}

	ts, err := m.chainAPI.ChainHead(ctx)
	if err != nil {
		return err
	}
	for ts.Height() > minHeight {
		for _, block := range ts.Blocks() {
			msgs, err := m.messageStore.SecpkMessagesForBlock(ctx, block)
			if err != nil {
				// If we can't find the messages, we've either imported from snapshot or pruned the store
				log.Debug("exiting message mapping population at epoch ", ts.Height())
				return nil
			}

			for _, msg := range msgs {
				m.ProcessSignedMessage(ctx, msg)
			}
		}

		var err error
		ts, err = m.chainAPI.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *ethTxHashManager) ProcessSignedMessage(ctx context.Context, msg *types.SignedMessage) {
	if msg.Signature.Type != crypto.SigTypeDelegated {
		return
	}

	ethTx, err := newEthTxFromSignedMessage(ctx, msg, m.chainAPI)
	if err != nil {
		log.Errorf("error converting filecoin message to eth tx: %s", err)
		return
	}

	err = m.TransactionHashLookup.UpsertHash(ethTx.Hash, msg.Cid())
	if err != nil {
		log.Errorf("error inserting tx mapping to db: %s", err)
		return
	}
}

func waitForMpoolUpdates(ctx context.Context, ch <-chan types.MpoolUpdate, manager *ethTxHashManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-ch:
			if u.Type != types.MpoolAdd {
				continue
			}

			manager.ProcessSignedMessage(ctx, u.Message)
		}
	}
}

func ethTxHashGC(ctx context.Context, retentionDays int, manager *ethTxHashManager) {
	if retentionDays == 0 {
		return
	}

	gcPeriod := 1 * time.Hour
	for {
		entriesDeleted, err := manager.TransactionHashLookup.DeleteEntriesOlderThan(retentionDays)
		if err != nil {
			log.Errorf("error garbage collecting eth transaction hash database: %s", err)
		}
		log.Info("garbage collection run on eth transaction hash lookup database. %d entries deleted", entriesDeleted)
		time.Sleep(gcPeriod)
	}
}

func parseEthTopics(topics types.EthTopicSpec) (map[string][][]byte, error) {
	keys := map[string][][]byte{}
	for idx, vals := range topics {
		if len(vals) == 0 {
			continue
		}
		// Ethereum topics are emitted using `LOG{0..4}` opcodes resulting in topics1..4
		key := fmt.Sprintf("t%d", idx+1)
		for _, v := range vals {
			v := v // copy the ethhash to avoid repeatedly referencing the same one.
			keys[key] = append(keys[key], v[:])
		}
	}
	return keys, nil
}

const errorFunctionSelector = "\x08\xc3\x79\xa0" // Error(string)
const panicFunctionSelector = "\x4e\x48\x7b\x71" // Panic(uint256)
// Eth ABI (solidity) panic codes.
var panicErrorCodes = map[uint64]string{
	0x00: "Panic()",
	0x01: "Assert()",
	0x11: "ArithmeticOverflow()",
	0x12: "DivideByZero()",
	0x21: "InvalidEnumVariant()",
	0x22: "InvalidStorageArray()",
	0x31: "PopEmptyArray()",
	0x32: "ArrayIndexOutOfBounds()",
	0x41: "OutOfMemory()",
	0x51: "CalledUninitializedFunction()",
}

// Parse an ABI encoded revert reason. This reason should be encoded as if it were the parameters to
// an `Error(string)` function call.
//
// See https://docs.soliditylang.org/en/latest/control-structures.html#panic-via-assert-and-error-via-require
func parseEthRevert(ret []byte) string {
	if len(ret) == 0 {
		return "none"
	}
	var cbytes abi.CborBytes
	if err := cbytes.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
		return "ERROR: revert reason is not cbor encoded bytes"
	}
	if len(cbytes) == 0 {
		return "none"
	}
	// If it's not long enough to contain an ABI encoded response, return immediately.
	if len(cbytes) < 4+32 {
		return types.EthBytes(cbytes).String()
	}
	switch string(cbytes[:4]) {
	case panicFunctionSelector:
		cbytes := cbytes[4 : 4+32]
		// Read the and check the code.
		code, err := types.EthUint64FromBytes(cbytes)
		if err != nil {
			// If it's too big, just return the raw value.
			codeInt := big.PositiveFromUnsignedBytes(cbytes)
			return fmt.Sprintf("Panic(%s)", types.EthBigInt(codeInt).String())
		}
		if s, ok := panicErrorCodes[uint64(code)]; ok {
			return s
		}
		return fmt.Sprintf("Panic(0x%x)", code)
	case errorFunctionSelector:
		cbytes := cbytes[4:]
		cbytesLen := types.EthUint64(len(cbytes))
		// Read the and check the offset.
		offset, err := types.EthUint64FromBytes(cbytes[:32])
		if err != nil {
			break
		}
		if cbytesLen < offset {
			break
		}

		// Read and check the length.
		if cbytesLen-offset < 32 {
			break
		}
		start := offset + 32
		length, err := types.EthUint64FromBytes(cbytes[offset : offset+32])
		if err != nil {
			break
		}
		if cbytesLen-start < length {
			break
		}
		// Slice the error message.
		return fmt.Sprintf("Error(%s)", cbytes[start:start+length])
	}
	return types.EthBytes(cbytes).String()
}

func calculateRewardsAndGasUsed(rewardPercentiles []float64, txGasRewards gasRewardSorter) ([]types.EthBigInt, uint64) {
	var totalGasUsed uint64
	for _, tx := range txGasRewards {
		totalGasUsed += tx.gas
	}

	rewards := make([]types.EthBigInt, len(rewardPercentiles))
	for i := range rewards {
		rewards[i] = types.EthBigIntZero
	}

	if len(txGasRewards) == 0 {
		return rewards, totalGasUsed
	}

	sort.Stable(txGasRewards)

	var idx int
	var sum uint64
	for i, percentile := range rewardPercentiles {
		threshold := uint64(float64(totalGasUsed) * percentile / 100)
		for sum < threshold && idx < len(txGasRewards)-1 {
			sum += txGasRewards[idx].gas
			idx++
		}
		rewards[i] = txGasRewards[idx].reward
	}

	return rewards, totalGasUsed
}

type gasRewardTuple struct {
	gas    uint64
	reward types.EthBigInt
}

// sorted in ascending order
type gasRewardSorter []gasRewardTuple

func (g gasRewardSorter) Len() int { return len(g) }
func (g gasRewardSorter) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}
func (g gasRewardSorter) Less(i, j int) bool {
	return g[i].reward.Int.Cmp(g[j].reward.Int) == -1
}

var _ v1.IETH = &ethAPI{}
var _ ethAPIAdapter = &ethAPI{}
