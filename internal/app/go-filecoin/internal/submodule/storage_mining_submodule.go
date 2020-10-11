package submodule

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	sectorstorage "github.com/filecoin-project/go-filecoin/vendors/sector-storage"
	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/ffiwrapper"
	"github.com/filecoin-project/go-filecoin/vendors/sector-storage/stores"
	fsm "github.com/filecoin-project/go-filecoin/vendors/storage-sealing"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/ipfs/go-datastore"

	fsmchain "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_chain"
	fsmeventsconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_events"
	fsmnodeconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_node"
	fsmstorage "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/fsm_storage"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/sectors"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsampler"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/poster"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
)

// StorageMiningSubmodule enhances the `Node` with storage mining capabilities.
type StorageMiningSubmodule struct {
	started   bool
	startedLk sync.RWMutex

	// StorageMining is used by the miner to fill and seal sectors.
	PieceManager piecemanager.PieceManager

	// PoStGenerator generates election PoSts
	PoStGenerator postgenerator.PoStGenerator

	hs     *chainsampler.HeightThresholdScheduler
	fsm    *fsm.Sealing
	poster *poster.Poster
}

// NewStorageMiningSubmodule creates a new storage mining submodule.
func NewStorageMiningSubmodule(
	minerAddr address.Address,
	ds datastore.Batching,
	c *ChainSubmodule,
	m *MessagingSubmodule,
	mw *msg.Waiter,
	stateViewer *appstate.Viewer,
	sealProofType abi.RegisteredSealProof,
	r repo.Repo,
	postGeneratorOverride postgenerator.PoStGenerator,
) (*StorageMiningSubmodule, error) {
	chainThresholdScheduler := chainsampler.NewHeightThresholdScheduler(c.ChainReader)

	ccn := fsmchain.NewChainConnector(c.ChainReader)

	sdx := stores.NewIndex()

	fcg := ffiwrapper.Config{
		SealProofType: sealProofType,
	}

	scg := sectorstorage.SealerConfig{AllowPreCommit1: true, AllowPreCommit2: true, AllowCommit: true}

	mgr, err := sectorstorage.New(context.TODO(), fsmstorage.NewRepoStorageConnector(r), sdx, &fcg, scg, []string{}, nil)
	if err != nil {
		return nil, err
	}

	sid := sectors.NewPersistedSectorNumberCounter(ds)

	// FSM requires id address to work correctly. Resolve it now and hope it's stable
	//
	minerAddrID, err := resolveMinerAddress(context.TODO(), c, minerAddr, stateViewer)
	if err != nil {
		return nil, err
	}

	ncn := fsmnodeconnector.New(minerAddrID, mw, c.ChainReader, c.ActorState, m.Outbox, c.State)

	ppStart, err := getMinerProvingPeriod(c, minerAddr, stateViewer)
	if err != nil {
		return nil, err
	}

	pcp := fsm.NewBasicPreCommitPolicy(&ccn, abi.ChainEpoch(2*60*24), ppStart%miner.WPoStProvingPeriod)

	fsmConnector := fsmeventsconnector.New(chainThresholdScheduler, c.State)
	fsm := fsm.New(ncn, fsm.FeeConfig{
		MaxPreCommitGasFee: abi.NewTokenAmount(1_0000_0000),
		MaxCommitGasFee:    abi.NewTokenAmount(1_0000_0000), //todo impl fee config
	}, fsmConnector, minerAddrID, ds, mgr, sid, ffiwrapper.ProofVerifier, &pcp, nil, nil) //todo impl defficult to impl SectorStateNotifee

	bke := piecemanager.NewFiniteStateMachineBackEnd(fsm, sid)

	postProof, err := sealProofType.RegisteredWinningPoStProof()
	if err != nil {
		return nil, err
	}
	modu := &StorageMiningSubmodule{
		PieceManager: &bke,
		hs:           chainThresholdScheduler,
		fsm:          fsm,
		poster:       poster.NewPoster(minerAddr, m.Outbox, mgr, c.State, stateViewer, postProof, nil, mw), //todo impl fault tracker
	}

	// allow the caller to provide a thing which generates fake PoSts
	if postGeneratorOverride == nil {
		modu.PoStGenerator = mgr.Prover
	} else {
		modu.PoStGenerator = postGeneratorOverride
	}

	return modu, nil
}

// Start starts the StorageMiningSubmodule
func (s *StorageMiningSubmodule) Start(ctx context.Context) error {
	s.startedLk.Lock()
	defer s.startedLk.Unlock()

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
	s.startedLk.Lock()
	defer s.startedLk.Unlock()

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
func (s *StorageMiningSubmodule) HandleNewHead(ctx context.Context, newHead *block.TipSet) error {
	s.startedLk.RLock()
	defer s.startedLk.RUnlock()

	if !s.started {
		return nil
	}

	err := s.hs.HandleNewTipSet(ctx, newHead)
	if err != nil {
		return err
	}

	return s.poster.HandleNewHead(ctx, newHead)
}

func getMinerProvingPeriod(c *ChainSubmodule, minerAddr address.Address, viewer *appstate.Viewer) (abi.ChainEpoch, error) {
	tsk := c.ChainReader.GetHead()
	root, err := c.ChainReader.GetTipSetStateRoot(tsk)
	if err != nil {
		return 0, err
	}
	view := viewer.StateView(root, network.Version0)
	return view.MinerProvingPeriodStart(context.Background(), minerAddr)
}

func resolveMinerAddress(ctx context.Context, c *ChainSubmodule, minerAddr address.Address, viewer *appstate.Viewer) (address.Address, error) {
	tsk := c.ChainReader.GetHead()
	root, err := c.ChainReader.GetTipSetStateRoot(tsk)
	if err != nil {
		return address.Undef, err
	}
	view := viewer.StateView(root, network.Version0)
	return view.InitResolveAddress(ctx, minerAddr)
}
