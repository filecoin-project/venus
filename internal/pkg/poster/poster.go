package poster

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-storage-miner"
	spaabi "github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	storageminerconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_miner_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
)

// TODO: replace with state.NewStore
type actorStore struct {
	ctx context.Context
	cst.ChainStateReadWriter
}

func (as *actorStore) Context() context.Context {
	return as.ctx
}

var _ adt.Store = new(actorStore)

// Poster listens for changes to the chain head and generates and submits a PoSt if one is required.
type Poster struct {
	postMutex      sync.Mutex
	postCancel     context.CancelFunc
	scheduleCancel context.CancelFunc

	minerAddr        address.Address
	outbox           *message.Outbox
	sectorbuilder    sectorbuilder.Interface
	minerNode        *storageminerconnector.StorageMinerNodeConnector
	storageMiner     *storage.Miner
	heaviestTipsetCh chan interface{}
	chain            *cst.ChainStateReadWriter
	stateViewer      *appstate.Viewer
}

// NewPoster creates a Poster struct
func NewPoster(
	minerAddr address.Address,
	outbox *message.Outbox,
	sb sectorbuilder.Interface,
	connector *storageminerconnector.StorageMinerNodeConnector,
	miner *storage.Miner,
	htc chan interface{},
	chain *cst.ChainStateReadWriter,
	stateViewer *appstate.Viewer) *Poster {

	return &Poster{
		minerAddr:        minerAddr,
		outbox:           outbox,
		sectorbuilder:    sb,
		minerNode:        connector,
		storageMiner:     miner,
		heaviestTipsetCh: htc,
		chain:            chain,
		stateViewer:      stateViewer,
	}
}

// StartPoSting starts the posting scheduler
func (p *Poster) StartPoSting(ctx context.Context) {
	p.StopPoSting()
	ctx, p.scheduleCancel = context.WithCancel(ctx)

	go func() {
		for {
			select {
			case ts, ok := <-p.heaviestTipsetCh:
				if !ok {
					return
				}

				newHead, ok := ts.(block.TipSet)
				if !ok {
					log.Warn("non-tipset published on heaviest tipset channel")
					continue
				}

				err := p.startPoStIfNeeded(ctx, newHead)
				if err != nil {
					log.Error("error attempting fallback post", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopPoSting stops the posting scheduler if running and any outstanding PoSts.
func (p *Poster) StopPoSting() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.scheduleCancel != nil {
		p.postCancel()

		p.scheduleCancel()
		p.scheduleCancel = nil
	}
}

func (p *Poster) startPoStIfNeeded(ctx context.Context, newHead block.TipSet) error {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.postCancel != nil {
		// already posting
		return nil
	}

	tipsetHeight, err := newHead.Height()
	if err != nil {
		return err
	}

	root, err := p.chain.GetTipStateStateRoot(ctx, newHead.Key())
	if err != nil {
		return err
	}

	stateView := p.stateViewer.StateView(root)
	provingPeriodStart, _, _, err := stateView.MinerProvingPeriod(ctx, p.minerAddr)
	if err != nil {
		return nil
	}

	if uint64(provingPeriodStart) <= tipsetHeight {
		// it's not time to PoSt
		return nil
	}

	ctx, p.postCancel = context.WithCancel(ctx)
	go func() {
		defer p.cancelPost()

		sortedSectorInfo, err := p.getProvingSet(ctx, stateView)
		if err != nil {
			log.Error("error getting proving set", err)
			return
		}

		faults, err := stateView.MinerFaults(ctx, p.minerAddr)
		if err != nil {
			log.Error("error getting faults", err)
			return
		}

		challengeSeed, err := p.getChallengeSeed(ctx, uint64(provingPeriodStart))
		if err != nil {
			log.Error("error getting challenge seed", err)
			return
		}

		candidates, proof, err := p.sectorbuilder.GenerateFallbackPoSt(sortedSectorInfo, challengeSeed, faults)
		if err != nil {
			log.Error("error generating fallback PoSt", err)
			return
		}

		err = p.sendPoSt(ctx, stateView, newHead.Key(), candidates, proof)
		if err != nil {
			log.Error("error sending fallback PoSt", err)
			return
		}
	}()

	return nil
}

func (p *Poster) sendPoSt(ctx context.Context, stateView *appstate.View, tipKey block.TipSetKey, candidates []sectorbuilder.EPostCandidate, proof []byte) error {
	minerIDAddr, err := stateView.InitResolveAddress(ctx, p.minerAddr)
	if err != nil {
		return err
	}

	minerID, err := address.IDFromAddress(minerIDAddr)
	if err != nil {
		return err
	}

	poStCandidates := make([]spaabi.PoStCandidate, len(candidates))
	for i, candidate := range candidates {
		poStCandidates[i] = spaabi.PoStCandidate{
			RegisteredProof: spaabi.RegisteredProof_WinStackedDRG32GiBPoSt,
			PartialTicket:   spaabi.PartialTicket(candidate.PartialTicket[:]),
			SectorID:        spaabi.SectorID{Miner: spaabi.ActorID(minerID), Number: spaabi.SectorNumber(candidate.SectorID)},
			ChallengeIndex:  int64(candidate.SectorChallengeIndex),
		}
	}

	windowedPost := spaabi.OnChainPoStVerifyInfo{
		ProofType:  spaabi.RegisteredProof_StackedDRG32GiBPoSt,
		Candidates: poStCandidates,
		Proofs:     []spaabi.PoStProof{{ProofBytes: proof}},
	}
	surprisePostParams, err := abi.ToEncodedValues(windowedPost)
	if err != nil {
		return err
	}

	_, workerAddr, err := stateView.MinerControlAddresses(ctx, p.minerAddr)
	if err != nil {
		return err
	}

	_, _, err = p.outbox.Send(
		ctx,
		workerAddr,
		p.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsMiner.SubmitWindowedPoSt),
		surprisePostParams,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *Poster) getProvingSet(ctx context.Context, stateView *appstate.View) (sectorbuilder.SortedPublicSectorInfo, error) {
	return consensus.NewPowerTableView(stateView).SortedSectorInfos(ctx, p.minerAddr)
}

func (p *Poster) getChallengeSeed(ctx context.Context, challengeHeight uint64) ([32]byte, error) {
	var challengeSeed [32]byte

	randomness, err := p.chain.SampleRandomness(ctx, types.NewBlockHeight(challengeHeight))
	if err != nil {
		return challengeSeed, err
	}

	copy(challengeSeed[:], randomness)

	return challengeSeed, nil
}

func (p *Poster) cancelPost() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.postCancel != nil {
		p.postCancel()
		p.postCancel = nil
	}
}
