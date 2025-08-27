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

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v10/evm"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/venus/pkg/chain"
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
	"github.com/hashicorp/golang-lru/arc/v2"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	cbg "github.com/whyrusleeping/cbor-gen"
)

const maxEthFeeHistoryRewardPercentiles = 100

var log = logging.Logger("eth_api")

var ErrNullRound = errors.New("requested epoch was a null round")
var ErrUnsupported = errors.New("unsupported method")

func newEthAPI(em *EthSubModule) (*ethAPI, error) {
	a := &ethAPI{
		em:              em,
		chain:           em.chainModule.API(),
		mpool:           em.mpoolModule.API(),
		EthEventHandler: em.ethEventAPI,
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

	cfg := em.cfg.FevmConfig
	if cfg.EthBlkCacheSize > 0 {
		var err error
		a.EthBlkCache, err = arc.NewARC[cid.Cid, *types.EthBlock](cfg.EthBlkCacheSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create block cache: %w", err)
		}

		a.EthBlkTxCache, err = arc.NewARC[cid.Cid, *types.EthBlock](cfg.EthBlkCacheSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create block transaction cache: %w", err)
		}
	}

	return a, nil
}

type ethAPI struct {
	em                   *EthSubModule
	chain                v1.IChain
	mpool                v1.IMessagePool
	ethTxHashManager     *ethTxHashManager
	EthEventHandler      *ethEventAPI
	MaxFilterHeightRange abi.ChainEpoch

	EthBlkCache   *arc.ARCCache[cid.Cid, *types.EthBlock] // caches blocks by their CID but blocks only have the transaction hashes
	EthBlkTxCache *arc.ARCCache[cid.Cid, *types.EthBlock] // caches blocks along with full transaction payload by their CID
}

func (a *ethAPI) start(ctx context.Context) error {
	ev, err := events.NewEvents(ctx, a.chain)
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

func (a *ethAPI) countTipsetMsgs(_ context.Context, ts *types.TipSet) (int, error) {
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
		return types.EthBlock{}, fmt.Errorf("failed to get tipset by cid: %w", err)
	}
	return a.getBlockByTipset(ctx, ts, fullTxInfo, "EthGetBlockByHash:"+blkHash.String())
}

func (a *ethAPI) EthGetBlockByNumber(ctx context.Context, blkParam string, fullTxInfo bool) (types.EthBlock, error) {
	ts, err := getTipsetByBlockNumber(ctx, a.em.chainModule.ChainReader, blkParam, true)
	if err != nil {
		return types.EthBlock{}, err
	}
	return a.getBlockByTipset(ctx, ts, fullTxInfo, "EthGetBlockByNumber:"+blkParam)
}

func (a *ethAPI) getBlockByTipset(ctx context.Context, ts *types.TipSet, fullTxInfo bool, req string) (types.EthBlock, error) {
	cache := a.EthBlkCache
	if fullTxInfo {
		cache = a.EthBlkTxCache
	}

	// Attempt to retrieve the Ethereum block from cache
	cid, err := ts.Key().Cid()
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("failed to get tipset key cid: %w", err)
	}
	if cache != nil {
		if ethBlock, found := cache.Get(cid); found {
			if ethBlock != nil {
				return *ethBlock, nil
			}
			// Log and remove the nil entry from cache
			log.Errorw("nil value in eth block cache", "cid", cid, "requested as", req)
			cache.Remove(cid)
		}
	}

	// Generate an Ethereum block from the Filecoin tipset
	blk, err := newEthBlockFromFilecoinTipSet(ctx, ts, fullTxInfo, a.em.chainModule.MessageStore, a.em.chainModule.Stmgr)
	if err != nil {
		return types.EthBlock{}, fmt.Errorf("failed to create Ethereum block from Filecoin tipset: %w", err)
	}

	// Add the newly created block to the cache and return
	if cache != nil {
		cache.Add(cid, &blk)
	}
	return blk, nil
}

func (a *ethAPI) parseBlkParam(ctx context.Context, blkParam string, strict bool) (tipset *types.TipSet, err error) {
	if blkParam == "earliest" {
		return nil, errors.New("block param \"earliest\" is not supported")
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
			return nil, errors.New("cannot get parent tipset")
		}
		return parent, nil
	default:
		var num types.EthUint64
		err := num.UnmarshalJSON([]byte(`"` + blkParam + `"`))
		if err != nil {
			return nil, fmt.Errorf("cannot parse block number: %v", err)
		}
		if abi.ChainEpoch(num) > head.Height()-1 {
			return nil, errors.New("requested a future epoch (beyond 'latest')")
		}
		ts, err := a.chain.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(num), head.Key())
		if err != nil {
			return nil, fmt.Errorf("cannot get tipset at height: %v", num)
		}
		if strict && ts.Height() != abi.ChainEpoch(num) {
			return nil, ErrNullRound
		}
		return ts, nil
	}
}

func (a *ethAPI) EthGetTransactionByHash(ctx context.Context, txHash *types.EthHash) (*types.EthTx, error) {
	return a.EthGetTransactionByHashLimited(ctx, txHash, constants.LookbackNoLimit)
}

func (a *ethAPI) EthGetTransactionByHashLimited(ctx context.Context, txHash *types.EthHash, limit abi.ChainEpoch) (*types.EthTx, error) {
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
	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, limit, true)
	if err == nil && msgLookup != nil {
		tx, err := newEthTxFromMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.em.chainModule.ChainReader)
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
			// We only return pending eth-account messages because we can't guarantee
			// that the from/to addresses of other messages are conversable to 0x-style
			// addresses. So we just ignore them.
			//
			// This should be "fine" as anyone using an "Ethereum-centric" block
			// explorer shouldn't care about seeing pending messages from native
			// accounts.
			ethtx, err := types.EthTransactionFromSignedFilecoinMessage(p)
			if err != nil {
				return nil, fmt.Errorf("could not convert Filecoin message into tx: %w", err)
			}

			tx, err := ethtx.ToEthTx(p)
			if err != nil {
				return nil, fmt.Errorf("could not convert Eth transaction to EthTx: %w", err)
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
	hash, err := ethTxHashFromMessageCid(ctx, cid, a.em.chainModule.MessageStore)
	if hash == types.EmptyEthHash {
		// not found
		return nil, nil
	}

	return &hash, err
}

func (a *ethAPI) EthGetTransactionCount(ctx context.Context, sender types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthUint64, error) {
	addr, err := sender.ToFilecoinAddress()
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("invalid address: %w", err)

	}
	// Handle "pending" block parameter separately
	if blkParam.PredefinedBlock != nil && *blkParam.PredefinedBlock == "pending" {
		nonce, err := a.mpool.MpoolGetNonce(ctx, addr)
		if err != nil {
			return types.EthUint64(0), fmt.Errorf("failed to get nonce from mpool: %w", err)
		}
		return types.EthUint64(nonce), nil
	}

	// For all other cases, get the tipset based on the block parameter
	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blkParam)
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("failed to process block param: %v, %w", blkParam, err)
	}

	// Get the actor state at the specified tipset
	actor, err := a.em.chainModule.Stmgr.GetActorAt(ctx, addr, ts)
	if err != nil {
		if errors.Is(err, types.ErrActorNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to lookup actor %s: %w", sender, err)
	}

	// Handle EVM actor case
	if builtinactors.IsEvmActor(actor.Code) {
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

	return types.EthUint64(actor.Nonce), nil
}

func (a *ethAPI) EthGetTransactionReceipt(ctx context.Context, txHash types.EthHash) (*types.EthTxReceipt, error) {
	return a.EthGetTransactionReceiptLimited(ctx, txHash, constants.LookbackNoLimit)
}

func (a *ethAPI) EthGetTransactionReceiptLimited(ctx context.Context, txHash types.EthHash, limit abi.ChainEpoch) (*types.EthTxReceipt, error) {
	c, err := a.ethTxHashManager.TransactionHashLookup.GetCidFromHash(txHash)
	if err != nil {
		log.Debug("could not find transaction hash %s in lookup table", txHash.String())
	}

	// This isn't an eth transaction we have the mapping for, so let's look it up as a filecoin message
	if c == cid.Undef {
		c = txHash.ToCid()
	}

	msgLookup, err := a.chain.StateSearchMsg(ctx, types.EmptyTSK, c, limit, true)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup Eth Txn %s as %s: %w", txHash, c, err)
	}
	if msgLookup == nil {
		// This is the best we can do. We may just not have indexed this transaction, or we may have a
		// limit applied and not searched far back enough, but we don't have a way to go. Because
		// Ethereum tooling expects an empty response for transaction-not-found, we don't have a way of
		// differentiating between "can't find" and "doesn't exist".
		return nil, nil
	}

	tx, err := newEthTxFromMessageLookup(ctx, msgLookup, -1, a.em.chainModule.MessageStore, a.em.chainModule.ChainReader)
	if err != nil {
		return nil, nil
	}

	ts, err := a.em.chainModule.ChainReader.GetTipSet(ctx, msgLookup.TipSet)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup tipset %s when constructing the eth txn receipt: %w", msgLookup.TipSet, err)
	}

	// The tx is located in the parent tipset
	parentTS, err := a.em.chainModule.ChainReader.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, fmt.Errorf("failed to lookup tipset %s when constructing the eth txn receipt: %w", ts.Parents(), err)
	}

	baseFee := parentTS.Blocks()[0].ParentBaseFee

	receipt, err := newEthTxReceipt(ctx, tx, baseFee, msgLookup.Receipt, a.EthEventHandler)
	if err != nil {
		return nil, nil
	}

	return &receipt, nil
}

func (a *ethAPI) EthGetTransactionByBlockHashAndIndex(ctx context.Context, blkHash types.EthHash, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, ErrUnsupported
}

func (a *ethAPI) EthGetBlockReceipts(ctx context.Context, blockParam types.EthBlockNumberOrHash) ([]*types.EthTxReceipt, error) {
	return a.EthGetBlockReceiptsLimited(ctx, blockParam, constants.LookbackNoLimit)
}

func (a *ethAPI) EthGetBlockReceiptsLimited(ctx context.Context, blockParam types.EthBlockNumberOrHash, limit abi.ChainEpoch) ([]*types.EthTxReceipt, error) {
	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blockParam)
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset: %w", err)
	}

	head, err := a.chain.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get head: %v", err)
	}
	if limit > constants.LookbackNoLimit && ts.Height() < head.Height() {
		return nil, fmt.Errorf("tipset %s is older than the allowed lookback limit", ts.Key())
	}

	tsCid, err := ts.Key().Cid()
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset key cid: %w", err)
	}

	blkHash, err := types.EthHashFromCid(tsCid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse eth hash from cid: %w", err)
	}

	// Execute the tipset to get the receipts, messages, and events
	_, msgs, receipts, err := executeTipset(ctx, ts, a.em.chainModule.MessageStore, a.em.chainModule.Stmgr)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tipset: %w", err)
	}

	// Load the state tree
	state, err := a.em.chainModule.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to get state view: %w", err)
	}

	baseFee := ts.Blocks()[0].ParentBaseFee

	ethReceipts := make([]*types.EthTxReceipt, 0, len(msgs))
	for i, msg := range msgs {
		msg := msg

		tx, err := newEthTx(ctx, state, ts.Height(), tsCid, msg.Cid(), i, a.ethTxHashManager.messageStore)
		if err != nil {
			return nil, fmt.Errorf("failed to create EthTx: %w", err)
		}

		receipt, err := newEthTxReceipt(ctx, tx, baseFee, receipts[i], a.EthEventHandler)
		if err != nil {
			return nil, fmt.Errorf("failed to create Eth receipt: %w", err)
		}

		// Set the correct Ethereum block hash
		receipt.BlockHash = blkHash

		ethReceipts = append(ethReceipts, &receipt)
	}

	return ethReceipts, nil
}

func (a *ethAPI) EthGetTransactionByBlockNumberAndIndex(ctx context.Context, blkNum types.EthUint64, txIndex types.EthUint64) (types.EthTx, error) {
	return types.EthTx{}, ErrUnsupported
}

// EthGetCode returns string value of the compiled bytecode
func (a *ethAPI) EthGetCode(ctx context.Context, ethAddr types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error) {
	to, err := ethAddr.ToFilecoinAddress()
	if err != nil {
		return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
	}

	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blkParam)
	if err != nil {
		return nil, fmt.Errorf("failed to process block param: %v, %w", blkParam, err)
	}

	// StateManager.Call will panic if there is no parent
	if ts.Height() == 0 {
		return nil, errors.New("block param must not specify genesis block")
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
		Method:     builtin.MethodsEVM.GetBytecode,
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
		return nil, errors.New("no message receipt")
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

func (a *ethAPI) EthGetStorageAt(ctx context.Context, ethAddr types.EthAddress, position types.EthBytes, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error) {
	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blkParam)
	if err != nil {
		return nil, fmt.Errorf("failed to process block param: %v, %w", blkParam, err)
	}

	l := len(position)
	if l > 32 {
		return nil, errors.New("supplied storage key is too long")
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
		Method:     builtin.MethodsEVM.GetStorageAt,
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
		return nil, errors.New("no message receipt")
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

func (a *ethAPI) EthGetBalance(ctx context.Context, address types.EthAddress, blkParam types.EthBlockNumberOrHash) (types.EthBigInt, error) {
	filAddr, err := address.ToFilecoinAddress()
	if err != nil {
		return types.EthBigInt{}, err
	}

	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blkParam)
	if err != nil {
		return types.EthBigInt{}, fmt.Errorf("failed to process block param: %v, %w", blkParam, err)
	}

	_, view, err := a.em.chainModule.Stmgr.StateView(ctx, ts)
	if err != nil {
		return types.EthBigInt{}, fmt.Errorf("failed to compute tipset state: %w", err)
	}

	actor, err := view.LoadActor(ctx, filAddr)
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

func (a *ethAPI) EthSyncing(ctx context.Context) (types.EthSyncingResult, error) {
	state, err := a.em.syncAPI.SyncState(ctx)
	if err != nil {
		return types.EthSyncingResult{}, fmt.Errorf("failed calling SyncState: %w", err)
	}

	if len(state.ActiveSyncs) == 0 {
		return types.EthSyncingResult{}, errors.New("no active syncs, try again")
	}

	working := -1
	for i, ss := range state.ActiveSyncs {
		if ss.Stage == types.StageIdle {
			continue
		}
		working = i

	}
	if working == -1 {
		working = len(state.ActiveSyncs) - 1
	}

	ss := state.ActiveSyncs[working]
	if ss.Base == nil || ss.Target == nil {
		return types.EthSyncingResult{}, errors.New("missing syncing information, try again")
	}

	res := types.EthSyncingResult{
		DoneSync:      ss.Stage == types.StageSyncComplete,
		CurrentBlock:  types.EthUint64(ss.Height),
		StartingBlock: types.EthUint64(ss.Base.Height()),
		HighestBlock:  types.EthUint64(ss.Target.Height()),
	}

	return res, nil
}

func (a *ethAPI) EthFeeHistory(ctx context.Context, p jsonrpc.RawParams) (types.EthFeeHistory, error) {
	params, err := jsonrpc.DecodeParams[types.EthFeeHistoryParams](p)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("decoding params: %w", err)
	}
	if params.BlkCount > 1024 {
		return types.EthFeeHistory{}, errors.New("block count should be smaller than 1024")
	}
	rewardPercentiles := make([]float64, 0)
	if params.RewardPercentiles != nil {
		if len(*params.RewardPercentiles) > maxEthFeeHistoryRewardPercentiles {
			return types.EthFeeHistory{}, errors.New("length of the reward percentile array cannot be greater than 100")
		}
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

	ts, err := a.parseBlkParam(ctx, params.NewestBlkNum, false)
	if err != nil {
		return types.EthFeeHistory{}, fmt.Errorf("bad block parameter %s: %s", params.NewestBlkNum, err)
	}

	var (
		basefee         = ts.Blocks()[0].ParentBaseFee
		oldestBlkHeight = uint64(1)

		// NOTE: baseFeePerGas should include the next block after the newest of the returned range,
		//  because the next base fee can be inferred from the messages in the newest block.
		//  However, this is NOT the case in Filecoin due to deferred execution, so the best
		//  we can do is duplicate the last value.
		baseFeeArray      = []types.EthBigInt{types.EthBigInt(basefee)}
		rewardsArray      = make([][]types.EthBigInt, 0)
		gasUsedRatioArray = []float64{}
		blocksIncluded    int
	)

	for blocksIncluded < int(params.BlkCount) && ts.Height() > 0 {
		basefee = ts.Blocks()[0].ParentBaseFee
		msgs, rcpts, err := messagesAndReceipts(ctx, ts, a.em.chainModule.MessageStore, a.em.chainModule.Stmgr)
		if err != nil {
			return types.EthFeeHistory{}, fmt.Errorf("failed to retrieve messages and receipts for height %d: %w", ts.Height(), err)
		}

		txGasRewards := gasRewardSorter{}
		for i, msg := range msgs {
			effectivePremium := msg.VMMessage().EffectiveGasPremium(basefee)
			txGasRewards = append(txGasRewards, gasRewardTuple{
				premium: effectivePremium,
				gasUsed: rcpts[i].GasUsed,
			})
		}

		rewards, totalGasUsed := calculateRewardsAndGasUsed(rewardPercentiles, txGasRewards)
		maxGas := constants.BlockGasLimit * int64(len(ts.Blocks()))

		// arrays should be reversed at the end
		baseFeeArray = append(baseFeeArray, types.EthBigInt(basefee))
		gasUsedRatioArray = append(gasUsedRatioArray, float64(totalGasUsed)/float64(maxGas))

		rewardsArray = append(rewardsArray, rewards)
		oldestBlkHeight = uint64(ts.Height())
		blocksIncluded++

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
	return strconv.FormatInt(int64(types2.Eip155ChainID), 10), nil
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
	txArgs, err := types.ParseEthTransaction(rawTx)
	if err != nil {
		return types.EmptyEthHash, err
	}

	smsg, err := types.ToSignedFilecoinMessage(txArgs)
	if err != nil {
		return types.EmptyEthHash, err
	}

	_, err = a.mpool.MpoolPush(ctx, smsg)
	if err != nil {
		return types.EmptyEthHash, err
	}
	return types.EthHashFromTxBytes(rawTx), nil
}

func (a *ethAPI) applyMessage(ctx context.Context, msg *types.Message, tsk types.TipSetKey) (*types.InvocResult, error) {
	ts, err := a.chain.ChainGetTipSet(ctx, tsk)
	if err != nil {
		return nil, fmt.Errorf("failed to got tipset %v", err)
	}

	if ts.Height() > 0 {
		pts, err := a.chain.ChainGetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to find a non-forking epoch: %w", err)
		}
		// Check for expensive forks from the parents to the tipset, including nil tipsets
		if a.em.chainModule.Fork.HasExpensiveForkBetween(pts.Height(), ts.Height()+1) {
			return nil, fork.ErrExpensiveFork
		}
	}

	st, err := a.em.chainModule.ChainReader.GetTipSetStateRoot(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("cannot get tipset state: %w", err)
	}
	res, err := a.em.chainModule.Stmgr.ApplyOnStateWithGas(ctx, st, msg, ts)
	if err != nil {
		return nil, fmt.Errorf("ApplyWithGasOnState failed: %w", err)
	}

	if res.MsgRct.ExitCode.IsError() {
		reason := parseEthRevert(res.MsgRct.Return)
		return nil, fmt.Errorf("message execution failed: exit %s, revert reason: %s, vm error: %s", res.MsgRct.ExitCode, reason, res.Error)
	}
	return res, nil
}

func (a *ethAPI) EthEstimateGas(ctx context.Context, p jsonrpc.RawParams) (types.EthUint64, error) {
	params, err := jsonrpc.DecodeParams[types.EthEstimateGasParams](p)
	if err != nil {
		return types.EthUint64(0), fmt.Errorf("decoding params: %w", err)
	}

	msg, err := params.Tx.ToFilecoinMessage()
	if err != nil {
		return types.EthUint64(0), err
	}
	// Set the gas limit to the zero sentinel value, which makes
	// gas estimation actually run.
	msg.GasLimit = 0

	var ts *types.TipSet
	if params.BlkParam == nil {
		ts, err = a.chain.ChainHead(ctx)
		if err != nil {
			return types.EthUint64(0), err
		}
	} else {
		ts, err = getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, *params.BlkParam)
		if err != nil {
			return types.EthUint64(0), fmt.Errorf("failed to process block param: %v; %w", params.BlkParam, err)
		}
	}
	gassedMsg, err := a.mpool.GasEstimateMessageGas(ctx, msg, nil, ts.Key())
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

	applyTSMessages := true
	if os.Getenv("VENUS_SKIP_APPLY_TS_MESSAGE_CALL_WITH_GAS") == "1" {
		applyTSMessages = false
	}

	canSucceed := func(limit int64) (bool, error) {
		msg.GasLimit = limit

		res, err := smgr.CallWithGas(ctx, &msg, priorMsgs, ts, applyTSMessages)
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

func (a *ethAPI) EthCall(ctx context.Context, tx types.EthCall, blkParam types.EthBlockNumberOrHash) (types.EthBytes, error) {
	msg, err := tx.ToFilecoinMessage()
	if err != nil {
		return nil, fmt.Errorf("failed to convert ethcall to filecoin message: %w", err)
	}
	ts, err := getTipsetByEthBlockNumberOrHash(ctx, a.em.chainModule.ChainReader, blkParam)
	if err != nil {
		return nil, fmt.Errorf("failed to process block param: %v, %w", blkParam, err)
	}

	invokeResult, err := a.applyMessage(ctx, msg, ts.Key())
	if err != nil {
		return nil, err
	}

	if msg.To == builtin.EthereumAddressManagerActorAddr {
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

func (a *ethAPI) EthTraceBlock(ctx context.Context, blkNum string) ([]*types.EthTraceBlock, error) {
	ts, err := getTipsetByBlockNumber(ctx, a.em.chainModule.ChainReader, blkNum, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset: %w", err)
	}

	_, trace, err := a.em.chainModule.Stmgr.ExecutionTrace(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed when calling ExecutionTrace: %w", err)
	}

	state, err := a.em.chainModule.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to get state view: %w", err)
	}

	cid, err := ts.Key().Cid()
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset key cid: %w", err)
	}

	blkHash, err := types.EthHashFromCid(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to parse eth hash from cid: %w", err)
	}

	allTraces := make([]*types.EthTraceBlock, 0, len(trace))
	msgIdx := 0
	for _, ir := range trace {
		// ignore messages from system actor
		if ir.Msg.From == builtinactors.SystemActorAddr {
			continue
		}

		msgIdx++

		txHash, err := a.EthGetTransactionHashByCid(ctx, ir.MsgCid)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction hash by cid: %w", err)
		}
		if txHash == nil {
			return nil, fmt.Errorf("cannot find transaction hash for cid %s", ir.MsgCid)
		}

		env, err := baseEnvironment(ctx, ir.Msg.From, state)
		if err != nil {
			return nil, fmt.Errorf("when processing message %s: %w", ir.MsgCid, err)
		}

		err = buildTraces(env, []int{}, &ir.ExecutionTrace)
		if err != nil {
			return nil, fmt.Errorf("failed building traces for msg %s: %w", ir.MsgCid, err)
		}

		for _, trace := range env.traces {
			allTraces = append(allTraces, &types.EthTraceBlock{
				EthTrace:            trace,
				BlockHash:           blkHash,
				BlockNumber:         int64(ts.Height()),
				TransactionHash:     *txHash,
				TransactionPosition: msgIdx,
			})
		}
	}

	return allTraces, nil
}

func (a *ethAPI) EthTraceReplayBlockTransactions(ctx context.Context, blkNum string, traceTypes []string) ([]*types.EthTraceReplayBlockTransaction, error) {
	if len(traceTypes) != 1 || traceTypes[0] != "trace" {
		return nil, errors.New("only 'trace' is supported")
	}

	ts, err := getTipsetByBlockNumber(ctx, a.em.chainModule.ChainReader, blkNum, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get tipset: %w", err)
	}

	_, trace, err := a.em.chainModule.Stmgr.ExecutionTrace(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed when calling ExecutionTrace: %w", err)
	}

	state, err := a.em.chainModule.ChainReader.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to get state view: %w", err)
	}

	allTraces := make([]*types.EthTraceReplayBlockTransaction, 0, len(trace))
	for _, ir := range trace {
		// ignore messages from system actor
		if ir.Msg.From == builtinactors.SystemActorAddr {
			continue
		}

		txHash, err := a.EthGetTransactionHashByCid(ctx, ir.MsgCid)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction hash by cid: %w", err)
		}
		if txHash == nil {
			return nil, fmt.Errorf("cannot find transaction hash for cid %s", ir.MsgCid)
		}

		env, err := baseEnvironment(ctx, ir.Msg.From, state)
		if err != nil {
			return nil, fmt.Errorf("when processing message %s: %w", ir.MsgCid, err)
		}

		err = buildTraces(env, []int{}, &ir.ExecutionTrace)
		if err != nil {
			return nil, fmt.Errorf("failed building traces for msg %s: %w", ir.MsgCid, err)
		}

		var output []byte
		if len(env.traces) > 0 {
			switch r := env.traces[0].Result.(type) {
			case *types.EthCallTraceResult:
				output = r.Output
			case *types.EthCreateTraceResult:
				output = r.Code
			}
		}

		allTraces = append(allTraces, &types.EthTraceReplayBlockTransaction{
			Output:          output,
			TransactionHash: *txHash,
			Trace:           env.traces,
			StateDiff:       nil,
			VMTrace:         nil,
		})
	}

	return allTraces, nil
}

func (a *ethAPI) EthTraceTransaction(ctx context.Context, txHash string) ([]*types.EthTraceTransaction, error) {

	// convert from string to internal type
	ethTxHash, err := types.ParseEthHash(txHash)
	if err != nil {
		return nil, fmt.Errorf("cannot parse eth hash: %w", err)
	}

	tx, err := a.EthGetTransactionByHash(ctx, &ethTxHash)
	if err != nil {
		return nil, fmt.Errorf("cannot get transaction by hash: %w", err)
	}

	if tx == nil {
		return nil, errors.New("transaction not found")
	}

	// tx.BlockNumber is nil when the transaction is still in the mpool/pending
	if tx.BlockNumber == nil {
		return nil, errors.New("no trace for pending transactions")
	}

	blockTraces, err := a.EthTraceBlock(ctx, strconv.FormatUint(uint64(*tx.BlockNumber), 10))
	if err != nil {
		return nil, fmt.Errorf("cannot get trace for block: %w", err)
	}

	txTraces := make([]*types.EthTraceTransaction, 0, len(blockTraces))
	for _, blockTrace := range blockTraces {
		if blockTrace.TransactionHash == ethTxHash {
			// Create a new EthTraceTransaction from the block trace
			txTrace := types.EthTraceTransaction{
				EthTrace:            blockTrace.EthTrace,
				BlockHash:           blockTrace.BlockHash,
				BlockNumber:         blockTrace.BlockNumber,
				TransactionHash:     blockTrace.TransactionHash,
				TransactionPosition: blockTrace.TransactionPosition,
			}
			txTraces = append(txTraces, &txTrace)
		}
	}

	return txTraces, nil
}

func (a *ethAPI) EthTraceFilter(ctx context.Context, filter types.EthTraceFilterCriteria) ([]*types.EthTraceFilterResult, error) {
	// Define EthBlockNumberFromString as a private function within EthTraceFilter
	getEthBlockNumberFromString := func(ctx context.Context, block *string) (types.EthUint64, error) {
		head := a.em.chainModule.ChainReader.GetHead()

		blockValue := "latest"
		if block != nil {
			blockValue = *block
		}

		switch blockValue {
		case "earliest":
			return 0, errors.New("block param \"earliest\" is not supported")
		case "pending":
			return types.EthUint64(head.Height()), nil
		case "latest":
			parent, err := a.em.chainModule.ChainReader.GetTipSet(ctx, head.Parents())
			if err != nil {
				return 0, errors.New("cannot get parent tipset")
			}
			return types.EthUint64(parent.Height()), nil
		case "safe":
			latestHeight := head.Height() - 1
			safeHeight := latestHeight - types.SafeEpochDelay
			return types.EthUint64(safeHeight), nil
		default:
			blockNum, err := types.EthUint64FromHex(blockValue)
			if err != nil {
				return 0, fmt.Errorf("cannot parse fromBlock: %w", err)
			}
			return blockNum, err
		}
	}

	fromBlock, err := getEthBlockNumberFromString(ctx, filter.FromBlock)
	if err != nil {
		return nil, fmt.Errorf("cannot parse fromBlock: %w", err)
	}

	toBlock, err := getEthBlockNumberFromString(ctx, filter.ToBlock)
	if err != nil {
		return nil, fmt.Errorf("cannot parse toBlock: %w", err)
	}

	var results []*types.EthTraceFilterResult

	if filter.Count != nil {
		// If filter.Count is specified and it is 0, return an empty result set immediately.
		if *filter.Count == 0 {
			return []*types.EthTraceFilterResult{}, nil
		}

		// If filter.Count is specified and is greater than the EthTraceFilterMaxResults config return error
		if uint64(*filter.Count) > a.em.cfg.FevmConfig.EthTraceFilterMaxResults {
			return nil, fmt.Errorf("invalid response count, requested %d, maximum supported is %d", *filter.Count, a.em.cfg.FevmConfig.EthTraceFilterMaxResults)
		}
	}

	traceCounter := types.EthUint64(0)
	for blkNum := fromBlock; blkNum <= toBlock; blkNum++ {
		blockTraces, err := a.EthTraceBlock(ctx, strconv.FormatUint(uint64(blkNum), 10))
		if err != nil {
			if errors.Is(err, &types.ErrNullRound{}) {
				continue
			}
			return nil, fmt.Errorf("cannot get trace for block %d: %w", blkNum, err)
		}

		for _, _blockTrace := range blockTraces {
			// Create a copy of blockTrace to avoid pointer quirks
			blockTrace := *_blockTrace
			match, err := matchFilterCriteria(&blockTrace, filter.FromAddress, filter.ToAddress)
			if err != nil {
				return nil, fmt.Errorf("cannot match filter for block %d: %w", blkNum, err)
			}
			if !match {
				continue
			}
			traceCounter++
			if filter.After != nil && traceCounter <= *filter.After {
				continue
			}

			results = append(results, &types.EthTraceFilterResult{
				EthTrace:            blockTrace.EthTrace,
				BlockHash:           blockTrace.BlockHash,
				BlockNumber:         blockTrace.BlockNumber,
				TransactionHash:     blockTrace.TransactionHash,
				TransactionPosition: blockTrace.TransactionPosition,
			})

			// If Count is specified, limit the results
			if filter.Count != nil && types.EthUint64(len(results)) >= *filter.Count {
				return results, nil
			} else if filter.Count == nil && uint64(len(results)) > a.em.cfg.FevmConfig.EthTraceFilterMaxResults {
				return nil, fmt.Errorf("too many results, maximum supported is %d, try paginating requests with After and Count", a.em.cfg.FevmConfig.EthTraceFilterMaxResults)
			}
		}
	}

	return results, nil
}

// matchFilterCriteria checks if a trace matches the filter criteria.
func matchFilterCriteria(trace *types.EthTraceBlock, fromDecodedAddresses []types.EthAddress, toDecodedAddresses []types.EthAddress) (bool, error) {
	var traceTo types.EthAddress
	var traceFrom types.EthAddress

	switch trace.Type {
	case "call":
		action, ok := trace.Action.(*types.EthCallTraceAction)
		if !ok {
			return false, errors.New("invalid call trace action")
		}
		traceTo = action.To
		traceFrom = action.From
	case "create":
		result, okResult := trace.Result.(*types.EthCreateTraceResult)
		if !okResult {
			return false, errors.New("invalid create trace result")
		}

		action, okAction := trace.Action.(*types.EthCreateTraceAction)
		if !okAction {
			return false, errors.New("invalid create trace action")
		}

		if result.Address == nil {
			return false, errors.New("address is nil in create trace result")
		}

		traceTo = *result.Address
		traceFrom = action.From
	default:
		return false, fmt.Errorf("invalid trace type: %s", trace.Type)
	}

	// Match FromAddress
	if len(fromDecodedAddresses) > 0 {
		fromMatch := false
		for _, ethAddr := range fromDecodedAddresses {
			if traceFrom == ethAddr {
				fromMatch = true
				break
			}
		}
		if !fromMatch {
			return false, nil
		}
	}

	// Match ToAddress
	if len(toDecodedAddresses) > 0 {
		toMatch := false
		for _, ethAddr := range toDecodedAddresses {
			if traceTo == ethAddr {
				toMatch = true
				break
			}
		}
		if !toMatch {
			return false, nil
		}
	}

	return true, nil
}

func calculateRewardsAndGasUsed(rewardPercentiles []float64, txGasRewards gasRewardSorter) ([]types.EthBigInt, int64) {
	var gasUsedTotal int64
	for _, tx := range txGasRewards {
		gasUsedTotal += tx.gasUsed
	}

	rewards := make([]types.EthBigInt, len(rewardPercentiles))
	for i := range rewards {
		rewards[i] = types.EthBigInt(types.NewInt(messagepool.MinGasPremium))
	}

	if len(txGasRewards) == 0 {
		return rewards, gasUsedTotal
	}

	sort.Stable(txGasRewards)

	var idx int
	var sum int64
	for i, percentile := range rewardPercentiles {
		threshold := int64(float64(gasUsedTotal) * percentile / 100)
		for sum < threshold && idx < len(txGasRewards)-1 {
			sum += txGasRewards[idx].gasUsed
			idx++
		}
		rewards[i] = types.EthBigInt(txGasRewards[idx].premium)
	}

	return rewards, gasUsedTotal
}

func getSignedMessage(ctx context.Context, ms *chain.MessageStore, msgCid cid.Cid) (*types.SignedMessage, error) {
	smsg, err := ms.LoadSignedMessage(ctx, msgCid)
	if err != nil {
		// We couldn't find the signed message, it might be a BLS message, so search for a regular message.
		msg, err := ms.LoadUnsignedMessage(ctx, msgCid)
		if err != nil {
			return nil, fmt.Errorf("failed to find msg %s: %w", msgCid, err)
		}
		smsg = &types.SignedMessage{
			Message: *msg,
			Signature: crypto.Signature{
				Type: crypto.SigTypeBLS,
			},
		}
	}

	return smsg, nil
}

type gasRewardTuple struct {
	gasUsed int64
	premium abi.TokenAmount
}

// sorted in ascending order
type gasRewardSorter []gasRewardTuple

func (g gasRewardSorter) Len() int { return len(g) }
func (g gasRewardSorter) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}
func (g gasRewardSorter) Less(i, j int) bool {
	return g[i].premium.Int.Cmp(g[j].premium.Int) == -1
}

var _ v1.IETH = &ethAPI{}
var _ ethAPIAdapter = &ethAPI{}
