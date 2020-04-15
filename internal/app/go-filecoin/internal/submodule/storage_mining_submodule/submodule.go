package storage_mining_submodule

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"

	sectorstorage "github.com/filecoin-project/sector-storage"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/internal/submodule"

	"github.com/filecoin-project/sector-storage/ffiwrapper"

	"github.com/filecoin-project/sector-storage/stores"

	fsmeventsconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_events"

	fsmnodeconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_node"

	fsm "github.com/filecoin-project/storage-fsm"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"

	storageminerconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/storage_miner"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/poster"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
)

// StorageMiningSubmodule enhances the `Node` with storage mining capabilities.
type StorageMiningSubmodule struct {
	started bool

	// StorageMining is used by the miner to fill and seal sectors.
	PieceManager piecemanager.PieceManager

	// PoStGenerator generates election PoSts
	PoStGenerator postgenerator.PoStGenerator

	minerNode *storageminerconnector.StorageMinerNodeConnector
	fsm       *fsm.Sealing
	poster    *poster.Poster
}

// NewStorageMiningSubmodule creates a new storage mining submodule.
func NewStorageMiningSubmodule(
	minerAddr address.Address,
	ds datastore.Batching,
	c *submodule.ChainSubmodule,
	m *submodule.MessagingSubmodule,
	mw *msg.Waiter,
	w *submodule.WalletSubmodule,
	stateViewer *appstate.Viewer,
	sealProofType abi.RegisteredProof,
	postProofType abi.RegisteredProof,
	sectorDirPath string,
) (*StorageMiningSubmodule, error) {
	minerNode := storageminerconnector.NewStorageMinerNodeConnector(minerAddr, c.ChainReader, c.State, m.Outbox, mw, w.Signer, stateViewer)

	ccn := newChainConnector(c.ChainReader)

	sdx := stores.NewIndex()

	fcg := ffiwrapper.Config{
		SealProofType: sealProofType,
		PoStProofType: postProofType,
	}

	scg := sectorstorage.SealerConfig{AllowPreCommit1: true, AllowPreCommit2: true, AllowCommit: true}

	mgr, err := sectorstorage.New(context.TODO(), newSinglePathLocalStorage(sectorDirPath), sdx, &fcg, scg, []string{}, nil)
	if err != nil {
		return nil, err
	}

	sid := newPersistedSectorNumberCounter(ds)

	ncn := fsmnodeconnector.New(mw, c.ChainReader, c.ActorState, m.Outbox, c.State)

	pcp := fsm.NewBasicPreCommitPolicy(&ccn, abi.ChainEpoch(2*60*24))

	fsmConnector := fsmeventsconnector.New(chainsampler.NewHeightThresholdScheduler(c.ChainReader), c.State)
	fsm := fsm.New(ncn, fsmConnector, minerAddr, ds, mgr, sid, ffiwrapper.ProofVerifier, ncn.ChainGetTicket, &pcp)

	bke := piecemanager.NewFiniteStateMachineBackEnd(fsm, sid)

	modu := &StorageMiningSubmodule{
		PieceManager:  &bke,
		PoStGenerator: mgr,
		minerNode:     minerNode,
		fsm:           fsm,
		poster:        poster.NewPoster(minerAddr, m.Outbox, mgr, c.State, stateViewer, mw),
	}

	return modu, nil
}

// Start starts the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Start(ctx context.Context) error {
	if s.started {
		return nil
	}

	err := s.fsm.Run(ctx)
	if err != nil {
		return err
	}

	s.started = true
	return nil
}

// Stop stops the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Stop(ctx context.Context) error {
	if !s.started {
		return nil
	}

	err := s.fsm.Stop(ctx)
	if err != nil {
		return err
	}

	s.poster.StopPoSting()
	s.started = false
	return nil
}

// HandleNewHead submits a new chain head for possible fallback PoSt.
func (s *StorageMiningSubmodule) HandleNewHead(ctx context.Context, newHead block.TipSet) error {
	if !s.started {
		return nil
	}

	err := s.minerNode.HandleNewTipSet(ctx, newHead)
	if err != nil {
		return err
	}

	return s.poster.HandleNewHead(ctx, newHead)
}
