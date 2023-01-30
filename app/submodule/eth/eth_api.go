package eth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/eam"
	"github.com/filecoin-project/go-state-types/builtin/v10/evm"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/ethhashlookup"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors"
	types2 "github.com/filecoin-project/venus/venus-shared/actors/types"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
)

var log = logging.Logger("eth_api")

func newEthAPI(em *EthSubModule) *ethAPI {
	return &ethAPI{
		em:               em,
		chain:            em.chainModule.API(),
		mpool:            em.mpoolModule.API(),
		ethTxHashManager: em.ethTxHashManager,
	}
}

type ethAPI struct {
	em               *EthSubModule
	chain            v1.IChain
	mpool            v1.IMessagePool
	ethTxHashManager *ethTxHashManager // may nil
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

func (a *ethAPI) EthGetTransactionHashByCid(ctx context.Context, cid cid.Cid) (*types.EthHash, error) {
	hash, err := ethTxHashFromFilecoinMessageCid(ctx, cid, a.em.chainModule.MessageStore, a.chain)
	return &hash, err
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

	c := cid.Undef
	if a.ethTxHashManager != nil {
		var err error
		c, err = a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(*txHash)
		if err != nil {
			log.Debug("could not find transaction hash %s in lookup table", txHash.String())
		}
	}
	// This isn't an eth transaction we have the mapping for, so let's look it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	// first, try to get the cid from mined transactions
	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, constants.LookbackNoLimit, true)
	if err == nil {
		tx, err := newEthTxFromFilecoinMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.chain)
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
			tx, err := newEthTxFromFilecoinMessage(ctx, p, a.chain)
			if err != nil {
				return nil, fmt.Errorf("could not convert Filecoin message into tx: %v", err)
			}
			return &tx, nil
		}
	}
	// Ethereum clients expect an empty response when the message was not found
	return nil, nil
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
	nonce, err := a.em.mpoolModule.MPool.GetNonce(ctx, addr, ts.Key())
	if err != nil {
		return types.EthUint64(0), err
	}
	return types.EthUint64(nonce), nil
}

func (a *ethAPI) EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error) {
	c := cid.Undef
	if a.ethTxHashManager != nil {
		var err error
		c, err = a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(txHash)
		if err != nil {
			log.Debug("could not find transaction hash %s in lookup table", txHash.String())
		}
	}
	// This isn't an eth transaction we have the mapping for, so let's look it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, constants.LookbackNoLimit, true)
	if err != nil || msgLookup == nil {
		return nil, nil
	}

	tx, err := newEthTxFromFilecoinMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.chain)
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

	// use the system actor as the caller
	from, err := address.NewIDAddress(0)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system sender address: %w", err)
	}
	msg := &types.Message{
		From:       from,
		To:         to,
		Value:      big.Zero(),
		Method:     builtintypes.MethodsEVM.GetBytecode,
		Params:     nil,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}

	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return nil, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	// Try calling until we find a height with no migration.
	var res *vm.Ret
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
		// if the call resulted in error, this is not an EVM smart contract;
		// return no bytecode.
		return nil, nil
	}

	if res.Receipt.ExitCode.IsError() {
		return nil, fmt.Errorf("message execution failed: exit %s, reason: %s", &res.Receipt.ExitCode, res.ActorErr)
	}

	var bytecodeCid cbg.CborCid
	if err := bytecodeCid.UnmarshalCBOR(bytes.NewReader(res.Receipt.Return)); err != nil {
		return nil, fmt.Errorf("failed to decode EVM bytecode CID: %w", err)
	}

	blk, err := a.em.chainModule.ChainReader.Blockstore().Get(ctx, cid.Cid(bytecodeCid))
	if err != nil {
		return nil, fmt.Errorf("failed to get EVM bytecode: %w", err)
	}

	return blk.RawData(), nil
}

func (a *ethAPI) EthGetStorageAt(ctx context.Context, ethAddr types.EthAddress, position types.EthBytes, blkParam string) (types.EthBytes, error) {
	l := len(position)
	if l > 32 {
		return nil, fmt.Errorf("supplied storage key is too long")
	}

	// pad with zero bytes if smaller than 32 bytes
	position = append(make([]byte, 32-l), position...)

	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	// use the system actor as the caller
	from, err := address.NewIDAddress(0)
	if err != nil {
		return nil, fmt.Errorf("failed to construct system sender address: %w", err)
	}

	// TODO super duper hack (raulk). The EVM runtime actor uses the U256 parameter type in
	//  GetStorageAtParams, which serializes as a hex-encoded string. It should serialize
	//  as bytes. We didn't get to fix in time for Iron, so for now we just pass
	//  through the hex-encoded value passed through the Eth JSON-RPC API, by remarshalling it.
	//  We don't fix this at origin (builtin-actors) because we are not updating the bundle
	//  for Iron.
	tmp, err := position.MarshalJSON()
	if err != nil {
		panic(err)
	}
	params, err := actors.SerializeParams(&evm.GetStorageAtParams{
		StorageKey: tmp[1 : len(tmp)-1], // TODO strip the JSON-encoding quotes -- yuck
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

	ts, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to got head %v", err)
	}

	// Try calling until we find a height with no migration.
	var res *vm.Ret
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

	return res.Receipt.Return, nil
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

func (a *ethAPI) EthFeeHistory(ctx context.Context, blkCount types.EthUint64, newestBlkNum string, rewardPercentiles []float64) (types.EthFeeHistory, error) {
	if blkCount > 1024 {
		return types.EthFeeHistory{}, fmt.Errorf("block count should be smaller than 1024")
	}

	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("failed to got head %v", err)
	}
	newestBlkHeight := uint64(head.Height())

	// TODO https://github.com/filecoin-project/ref-fvm/issues/1016
	var blkNum types.EthUint64
	err = blkNum.UnmarshalJSON([]byte(`"` + newestBlkNum + `"`))
	if err == nil && uint64(blkNum) < newestBlkHeight {
		newestBlkHeight = uint64(blkNum)
	}

	// Deal with the case that the chain is shorter than the number of
	// requested blocks.
	oldestBlkHeight := uint64(1)
	if uint64(blkCount) <= newestBlkHeight {
		oldestBlkHeight = newestBlkHeight - uint64(blkCount) + 1
	}

	ts, err := a.em.chainModule.ChainReader.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(newestBlkHeight), false)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("cannot load find block height: %v", newestBlkHeight)
	}

	// FIXME: baseFeePerGas should include the next block after the newest of the returned range, because this
	// can be inferred from the newest block. we use the newest block's baseFeePerGas for now but need to fix it
	// In other words, due to deferred execution, we might not be returning the most useful value here for the client.
	baseFeeArray := []types.EthBigInt{types.EthBigInt(ts.Blocks()[0].ParentBaseFee)}
	gasUsedRatioArray := []float64{}

	for ts.Height() >= abi.ChainEpoch(oldestBlkHeight) {
		// Unfortunately we need to rebuild the full message view so we can
		// totalize gas used in the tipset.
		block, err := newEthBlockFromFilecoinTipSet(ctx, ts, false, a.em.chainModule.MessageStore, a.chain)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("cannot create eth block: %v", err)
		}

		// both arrays should be reversed at the end
		baseFeeArray = append(baseFeeArray, types.EthBigInt(ts.Blocks()[0].ParentBaseFee))
		gasUsedRatioArray = append(gasUsedRatioArray, float64(block.GasUsed)/float64(constants.BlockGasLimit))

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

	return types.EthFeeHistory{
		OldestBlock:   oldestBlkHeight,
		BaseFeePerGas: baseFeeArray,
		GasUsedRatio:  gasUsedRatioArray,
	}, nil
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

	_, err = a.chain.StateGetActor(ctx, smsg.Message.To, types.EmptyTSK)
	if err != nil {
		// if actor does not exist on chain yet, set the method to 0 because
		// placeholders only implement method 0
		smsg.Message.Method = builtin.MethodSend
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
	var to address.Address
	var method abi.MethodNum
	if tx.To == nil {
		// this is a contract creation
		to = builtintypes.EthereumAddressManagerActorAddr

		initcode := abi.CborBytes(tx.Data)
		params2, err := actors.SerializeParams(&initcode)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize Create params: %w", err)
		}
		params = params2
		method = builtintypes.MethodsEAM.CreateExternal
	} else {
		addr, err := tx.To.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
		}
		to = addr

		if len(tx.Data) > 0 {
			var buf bytes.Buffer
			if err := cbg.WriteByteArray(&buf, tx.Data); err != nil {
				return nil, fmt.Errorf("failed to encode tx input into a cbor byte-string")
			}
			params = buf.Bytes()
			method = builtintypes.MethodsEVM.InvokeContract
		} else {
			method = builtintypes.MethodSend
		}
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
	var res *vmcontext.Ret
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
	if res.Receipt.ExitCode.IsError() {
		return nil, fmt.Errorf("message execution failed: exit %s, msg receipt return: %s, reason: %v", res.Receipt.ExitCode, res.Receipt.Return, res.ActorErr)
	}
	return &types.InvocResult{
		MsgCid:         msg.Cid(),
		Msg:            msg,
		MsgRct:         &res.Receipt,
		ExecutionTrace: res.GasTracker.ExecutionTrace,
		Duration:       res.Duration,
	}, nil
}

func (a *ethAPI) EthEstimateGas(ctx context.Context, tx types.EthCall) (types.EthUint64, error) {
	msg, err := a.ethCallToFilecoinMessage(ctx, tx)
	if err != nil {
		return types.EthUint64(0), err
	}
	// Set the gas limit to the zero sentinel value, which makes
	// gas estimation actually run.
	msg.GasLimit = 0

	msg, err = a.mpool.GasEstimateMessageGas(ctx, msg, nil, types.EmptyTSK)
	if err != nil {
		return types.EthUint64(0), err
	}

	return types.EthUint64(msg.GasLimit), nil
}

func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam string) (types.EthBytes, error) {
	msg, err := a.ethCallToFilecoinMessage(ctx, tx)
	if err != nil {
		return nil, err
	}
	ts, err := a.parseBlkParam(ctx, blkParam)
	if err != nil {
		return nil, fmt.Errorf("cannot parse block param: %s", blkParam)
	}

	invokeResult, err := a.applyMessage(ctx, msg, ts.Key())
	if err != nil {
		return nil, err
	}
	if len(invokeResult.MsgRct.Return) > 0 {
		return cbg.ReadByteArray(bytes.NewReader(invokeResult.MsgRct.Return), uint64(len(invokeResult.MsgRct.Return)))
	}
	return types.EthBytes{}, nil
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

	block := types.NewEthBlock()

	// this seems to be a very expensive way to get gasUsed of the block. may need to find an efficient way to do it
	gasUsed := int64(0)
	for txIdx, msg := range msgs {
		msgLookup, err := ca.StateSearchMsg(ctx, types.EmptyTSK, msg.Cid(), constants.LookbackNoLimit, false)
		if err != nil || msgLookup == nil {
			return types.EthBlock{}, nil
		}
		gasUsed += msgLookup.Receipt.GasUsed

		tx, err := newEthTxFromFilecoinMessageLookup(ctx, msgLookup, txIdx, ms, ca)
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

func ethTxHashFromFilecoinMessageCid(ctx context.Context, c cid.Cid, ms *chain.MessageStore, ca v1.IChain) (types.EthHash, error) {
	smsg, err := ms.LoadSignedMessage(ctx, c)
	if err == nil {
		return ethTxHashFromSignedFilecoinMessage(ctx, smsg, ca)
	}

	return types.EthHashFromCid(c)
}

func ethTxHashFromSignedFilecoinMessage(ctx context.Context, smsg *types.SignedMessage, ca v1.IChain) (types.EthHash, error) {
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		ethTx, err := newEthTxFromFilecoinMessage(ctx, smsg, ca)
		if err != nil {
			return types.EmptyEthHash, err
		}
		return ethTx.Hash, nil
	}

	return types.EthHashFromCid(smsg.Cid())
}

func newEthTxFromFilecoinMessage(ctx context.Context, smsg *types.SignedMessage, ca v1.IChain) (types.EthTx, error) {
	// Ignore errors here so we can still parse non-eth messages
	fromEthAddr, _ := lookupEthAddress(ctx, smsg.Message.From, ca)
	toEthAddr, _ := lookupEthAddress(ctx, smsg.Message.To, ca)

	toAddr := &toEthAddr
	input := smsg.Message.Params
	var err error
	// Check to see if we need to decode as contract deployment.
	// We don't need to resolve the to address, because there's only one form (an ID).
	if smsg.Message.To == builtintypes.EthereumAddressManagerActorAddr {
		switch smsg.Message.Method {
		case builtintypes.MethodsEAM.Create:
			toAddr = nil
			var params eam.CreateParams
			err = params.UnmarshalCBOR(bytes.NewReader(smsg.Message.Params))
			input = params.Initcode
		case builtintypes.MethodsEAM.Create2:
			toAddr = nil
			var params eam.Create2Params
			err = params.UnmarshalCBOR(bytes.NewReader(smsg.Message.Params))
			input = params.Initcode
		case builtintypes.MethodsEAM.CreateExternal:
			toAddr = nil
			var params abi.CborBytes
			err = params.UnmarshalCBOR(bytes.NewReader(smsg.Message.Params))
			input = []byte(params)
		}
		if err != nil {
			return types.EthTx{}, err
		}
	}
	// Otherwise, try to decode as a cbor byte array.
	// TODO: Actually check if this is an ethereum call. This code will work for demo purposes, but is not correct.
	if toAddr != nil {
		if decodedParams, err := cbg.ReadByteArray(bytes.NewReader(smsg.Message.Params), uint64(len(smsg.Message.Params))); err == nil {
			input = decodedParams
		}
	}

	r, s, v, err := types.RecoverSignature(smsg.Signature)
	if err != nil {
		// we don't want to return error if the message is not an Eth tx
		r, s, v = types.EthBigIntZero, types.EthBigIntZero, types.EthBigIntZero
	}

	tx := types.EthTx{
		ChainID:              types.EthUint64(types2.Eip155ChainID),
		From:                 fromEthAddr,
		To:                   toAddr,
		Value:                types.EthBigInt(smsg.Message.Value),
		Type:                 types.EthUint64(2),
		Input:                input,
		Gas:                  types.EthUint64(smsg.Message.GasLimit),
		MaxFeePerGas:         types.EthBigInt(smsg.Message.GasFeeCap),
		MaxPriorityFeePerGas: types.EthBigInt(smsg.Message.GasPremium),
		V:                    v,
		R:                    r,
		S:                    s,
	}

	// This is an eth tx
	if smsg.Signature.Type == crypto.SigTypeDelegated {
		tx.Hash, err = tx.TxHash()
		if err != nil {
			return tx, err
		}
	} else if smsg.Signature.Type == crypto.SigTypeUnknown { // BLS Filecoin message
		tx.Hash, err = types.EthHashFromCid(smsg.Message.Cid())
		if err != nil {
			return tx, err
		}
	} else { // Secp Filecoin Message
		tx.Hash, err = types.EthHashFromCid(smsg.Cid())
		if err != nil {
			return tx, err
		}
	}

	return tx, nil
}

// newEthTxFromFilecoinMessageLookup creates an ethereum transaction from filecoin message lookup. If a negative txIdx is passed
// into the function, it looksup the transaction index of the message in the tipset, otherwise it uses the txIdx passed into the
// function
func newEthTxFromFilecoinMessageLookup(ctx context.Context, msgLookup *types.MsgLookup, txIdx int, ms *chain.MessageStore, ca v1.IChain) (types.EthTx, error) {
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

	tx, err := newEthTxFromFilecoinMessage(ctx, smsg, ca)
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

	if receipt.To == nil && lookup.Receipt.ExitCode.IsSuccess() {
		// Create and Create2 return the same things.
		var ret eam.CreateReturn
		if err := ret.UnmarshalCBOR(bytes.NewReader(lookup.Receipt.Return)); err != nil {
			return types.EthTxReceipt{}, fmt.Errorf("failed to parse contract creation result: %w", err)
		}
		addr := types.EthAddress(ret.EthAddress)
		receipt.ContractAddress = &addr
	}

	if lookup.Receipt.ExitCode.IsSuccess() {
		receipt.Status = 1
	}
	if lookup.Receipt.ExitCode.IsError() {
		receipt.Status = 0
	}

	if len(events) > 0 {
		// TODO return a dummy non-zero bloom to signal that there are logs
		//  need to figure out how worth it is to populate with a real bloom
		//  should be feasible here since we are iterating over the logs anyway
		receipt.LogsBloom[255] = 0x01

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

			for _, entry := range evt.Entries {
				value := types.EthBytes(leftpad32(entry.Value)) // value has already been cbor-decoded but see https://github.com/filecoin-project/ref-fvm/issues/1345
				if entry.Key == types.EthTopic1 || entry.Key == types.EthTopic2 || entry.Key == types.EthTopic3 || entry.Key == types.EthTopic4 {
					l.Topics = append(l.Topics, value)
				} else {
					l.Data = value
				}
			}

			addr, err := address.NewIDAddress(uint64(evt.Emitter))
			if err != nil {
				return types.EthTxReceipt{}, fmt.Errorf("failed to create ID address: %w", err)
			}

			l.Address, err = lookupEthAddress(ctx, addr, ca)
			if err != nil {
				return types.EthTxReceipt{}, fmt.Errorf("failed to resolve Ethereum address: %w", err)
			}

			receipt.Logs = append(receipt.Logs, l)
		}
	}

	receipt.GasUsed = types.EthUint64(lookup.Receipt.GasUsed)

	// TODO: handle CumulativeGasUsed
	receipt.CumulativeGasUsed = types.EmptyEthInt

	effectiveGasPrice := big.Div(replay.GasCost.TotalCost, big.NewInt(lookup.Receipt.GasUsed))
	receipt.EffectiveGasPrice = types.EthBigInt(effectiveGasPrice)

	return receipt, nil
}

type ethTxHashManager struct {
	chainAPI              v1.IChain
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

			hash, err := ethTxHashFromSignedFilecoinMessage(ctx, smsg, m.chainAPI)
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

func waitForMpoolUpdates(ctx context.Context, ch <-chan types.MpoolUpdate, manager *ethTxHashManager) {
	for {
		select {
		case <-ctx.Done():
			return
		case u := <-ch:
			if u.Type != types.MpoolAdd {
				continue
			}
			if u.Message.Signature.Type != crypto.SigTypeDelegated {
				continue
			}

			ethTx, err := newEthTxFromFilecoinMessage(ctx, u.Message, manager.chainAPI)
			if err != nil {
				log.Errorf("error converting filecoin message to eth tx: %s", err)
			}

			err = manager.TransactionHashLookup.UpsertHash(ethTx.Hash, u.Message.Cid())
			if err != nil {
				log.Errorf("error inserting tx mapping to db: %s", err)
			}
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
