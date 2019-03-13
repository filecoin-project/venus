package protocolapi

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
	apiDeps
}

type apiDeps struct {
	AddNewBlockFunc   func(context.Context, *types.Block) (err error)
	BlockStore        blockstore.Blockstore
	CBorStore         *hamt.CborIpldStore
	OnlineStore       *hamt.CborIpldStore
	ChainReader       chain.ReadStore
	ConsensusProtocol consensus.Protocol
	BlockTime         time.Duration
	MineDelay         time.Duration
	MsgPool           *core.MessagePool
	NodePorcelain     *porcelain.API
	PowerTable        consensus.PowerTableView
	Syncer            chain.Syncer
	TicketSigner      consensus.TicketSigner
}

// New creates a new API instance with the provided deps
func New(
	addNewBlockFunc func(context.Context, *types.Block) (err error),
	bstore blockstore.Blockstore,
	cborStore, onlineStore *hamt.CborIpldStore,
	chainReader chain.ReadStore,
	con consensus.Protocol,
	blockTime, blockMineDelay time.Duration,
	msgPool *core.MessagePool,
	nodePorc *porcelain.API,
	ptv consensus.PowerTableView,
	syncer chain.Syncer,
	signer consensus.TicketSigner,
) API {
	newDeps := apiDeps{
		AddNewBlockFunc:   addNewBlockFunc,
		BlockStore:        bstore,
		CBorStore:         cborStore,
		OnlineStore:       onlineStore,
		ChainReader:       chainReader,
		ConsensusProtocol: con,
		BlockTime:         blockTime,
		MineDelay:         blockMineDelay,
		MsgPool:           msgPool,
		NodePorcelain:     nodePorc,
		PowerTable:        ptv,
		Syncer:            syncer,
		TicketSigner:      signer,
	}

	return API{apiDeps: newDeps}
}

// MineOnce mines a single block in the given context, and returns the new block.
func (a *API) MineOnce(ctx context.Context) (*types.Block, error) {

	getStateByKey := func(ctx context.Context, tsKey string) (state.Tree, error) {
		tsas, err := a.ChainReader.GetTipSetAndState(ctx, tsKey)
		if err != nil {
			return nil, err
		}
		return state.LoadStateTree(ctx, a.CBorStore, tsas.TipSetStateRoot, builtin.Actors)
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
			return a.ConsensusProtocol.Weight(ctx, ts, nil)
		}
		pSt, err := getStateByKey(ctx, parent.String())
		if err != nil {
			return uint64(0), err
		}
		return a.ConsensusProtocol.Weight(ctx, ts, pSt)
	}

	minerAddrIf, err := a.NodePorcelain.ConfigGet("mining.minerAddress")
	if err != nil {
		return nil, err
	}
	minerAddr := minerAddrIf.(address.Address)
	minerOwnerAddr, err := a.NodePorcelain.MinerGetOwnerAddress(ctx, minerAddr)
	if err != nil {
		return nil, err
	}

	minerPubKey, err := a.NodePorcelain.MinerGetKey(ctx, minerAddr)
	if err != nil {
		return nil, err
	}

	getAncestors := func(ctx context.Context, ts types.TipSet, newBlockHeight *types.BlockHeight) ([]types.TipSet, error) {
		return chain.GetRecentAncestors(ctx, ts, a.ChainReader, newBlockHeight, consensus.AncestorRoundsNeeded, sampling.LookbackParameter)
	}

	worker := mining.NewDefaultWorker(
		a.MsgPool, getState, getWeight, getAncestors, consensus.NewDefaultProcessor(),
		a.PowerTable, a.BlockStore, a.CBorStore, minerAddr, minerOwnerAddr, minerPubKey,
		a.TicketSigner, a.BlockTime)

	ts := a.ChainReader.Head()
	res, err := mining.MineOnce(ctx, worker, a.MineDelay, ts)
	if err != nil {
		return nil, err
	}
	if res.Err != nil {
		return nil, res.Err
	}

	if err := a.AddNewBlockFunc(ctx, res.NewBlock); err != nil {
		return nil, err
	}

	return res.NewBlock, nil
}
