package block

import (
	"context"
	"time"

	"gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// API provides an interface to protocols.
type API struct {
	addNewBlockFunc   func(context.Context, *types.Block) (err error)
	blockStore        blockstore.Blockstore
	cborStore         *hamt.CborIpldStore
	chainReader       chain.ReadStore
	consensusProtocol consensus.Protocol
	blockTime         time.Duration
	mineDelay         time.Duration
	msgPool           *core.MessagePool
	nodePorcelain     *porcelain.API
	powerTable        consensus.PowerTableView
	startMiningFunc   func(context.Context) error
	stopMiningFunc    func(context.Context)
	syncer            chain.Syncer
	ticketSigner      consensus.TicketSigner
}

// New creates a new API instance with the provided deps
func New(
	addNewBlockFunc func(context.Context, *types.Block) (err error),
	bstore blockstore.Blockstore,
	cborStore *hamt.CborIpldStore,
	chainReader chain.ReadStore,
	con consensus.Protocol,
	blockTime, blockMineDelay time.Duration,
	msgPool *core.MessagePool,
	nodePorc *porcelain.API,
	ptv consensus.PowerTableView,
	startMiningFunc func(context.Context) error,
	stopMiningfunc func(context.Context),
	syncer chain.Syncer,
	signer consensus.TicketSigner,
) API {
	return API{
		addNewBlockFunc:   addNewBlockFunc,
		blockStore:        bstore,
		cborStore:         cborStore,
		chainReader:       chainReader,
		consensusProtocol: con,
		blockTime:         blockTime,
		mineDelay:         blockMineDelay,
		msgPool:           msgPool,
		nodePorcelain:     nodePorc,
		powerTable:        ptv,
		startMiningFunc:   startMiningFunc,
		stopMiningFunc:    stopMiningfunc,
		syncer:            syncer,
		ticketSigner:      signer,
	}
}

// MiningOnce mines a single block in the given context, and returns the new block.
func (a *API) MiningOnce(ctx context.Context) (*types.Block, error) {

	getStateByKey := func(ctx context.Context, tsKey string) (state.Tree, error) {
		tsas, err := a.chainReader.GetTipSetAndState(ctx, tsKey)
		if err != nil {
			return nil, err
		}
		return state.LoadStateTree(ctx, a.cborStore, tsas.TipSetStateRoot, builtin.Actors)
	}

	getState := func(ctx context.Context, ts types.TipSet) (state.Tree, error) {
		return getStateByKey(ctx, ts.String())
	}

	getWeight := func(ctx context.Context, ts types.TipSet) (uint64, error) {
		parent, err := ts.Parents()
		if err != nil {
			return uint64(0), err
		}
		if parent.Len() == 0 {
			return a.consensusProtocol.Weight(ctx, ts, nil)
		}
		pSt, err := getStateByKey(ctx, parent.String())
		if err != nil {
			return uint64(0), err
		}
		return a.consensusProtocol.Weight(ctx, ts, pSt)
	}

	minerAddrIf, err := a.nodePorcelain.ConfigGet("mining.minerAddress")
	if err != nil {
		return nil, err
	}
	minerAddr := minerAddrIf.(address.Address)
	minerOwnerAddr, err := a.nodePorcelain.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, err
	}

	minerPubKey, err := a.nodePorcelain.MinerGetKey(ctx, minerAddr)
	if err != nil {
		return nil, err
	}

	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return chain.GetRecentAncestors(ctx, ts, a.chainReader, newBlockHeight, consensus.AncestorRoundsNeeded, sampling.LookbackParameter)
	}

	worker := mining.NewDefaultWorker(
		a.msgPool, getState, getWeight, getAncestors, consensus.NewDefaultProcessor(),
		a.powerTable, a.blockStore, a.cborStore, minerAddr, minerOwnerAddr, minerPubKey,
		a.ticketSigner, a.blockTime)

	ts := a.chainReader.Head()
	res, err := mining.MineOnce(ctx, worker, a.mineDelay, ts)
	if err != nil {
		return nil, err
	}
	if res.Err != nil {
		return nil, res.Err
	}

	if err := a.addNewBlockFunc(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}

// MiningStart calls the node's StartMining function
func (a *API) MiningStart(ctx context.Context) error {
	return a.startMiningFunc(ctx)
}

// MiningStop calls the node's StopMining function
func (a *API) MiningStop(ctx context.Context) {
	a.stopMiningFunc(ctx)
}
