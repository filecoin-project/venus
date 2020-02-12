package poster

import (
	"context"
	"sync"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-address"
	commcid "github.com/filecoin-project/go-fil-commcid"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/go-storage-miner"
	spaabi "github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	storageminerconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_miner_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
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
}

// NewPoster creates a Poster struct
func NewPoster(
	minerAddr address.Address,
	outbox *message.Outbox,
	sb sectorbuilder.Interface,
	connector *storageminerconnector.StorageMinerNodeConnector,
	miner *storage.Miner,
	htc chan interface{},
	chain *cst.ChainStateReadWriter) *Poster {

	return &Poster{
		minerAddr:        minerAddr,
		outbox:           outbox,
		sectorbuilder:    sb,
		minerNode:        connector,
		storageMiner:     miner,
		heaviestTipsetCh: htc,
		chain:            chain,
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

				p.startPoStIfNeeded(ctx, newHead)
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

	var minerState miner.State
	err := p.chain.GetActorStateAt(ctx, newHead.Key(), p.minerAddr, &minerState)
	if err != nil {
		return err
	}

	if minerState.PoStState.SurpriseChallengeEpoch == 0 {
		// it's not time to PoSt
		return nil
	}

	ctx, p.postCancel = context.WithCancel(ctx)
	go func() {
		defer p.cancelPost()

		sortedSectorInfo, err := p.getProvingSet(ctx, minerState)
		if err != nil {
			log.Error("error getting proving set", err)
			return
		}

		faults, err := p.getFaults(ctx, minerState)
		if err != nil {
			log.Error("error getting faults", err)
			return
		}

		challengeSeed, err := p.getChallengeSeed(ctx, newHead)
		if err != nil {
			log.Error("error getting challenge seed", err)
			return
		}

		candidates, proof, err := p.sectorbuilder.GenerateFallbackPoSt(sortedSectorInfo, challengeSeed, faults)
		if err != nil {
			log.Error("error generating fallback PoSt", err)
			return
		}

		err = p.sendPoSt(ctx, minerState, newHead.Key(), candidates, proof)
		if err != nil {
			log.Error("error sending fallback PoSt", err)
			return
		}
	}()

	return nil
}

func (p *Poster) sendPoSt(ctx context.Context, minerState miner.State, tipKey block.TipSetKey, candidates []sectorbuilder.EPostCandidate, proof []byte) error {
	minerIDAddr, err := p.chain.ResolveAddressAt(ctx, tipKey, p.minerAddr)
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

	surprisePoSt := miner.SubmitSurprisePoStResponseParams{
		OnChainInfo: spaabi.OnChainSurprisePoStVerifyInfo{
			RegisteredProof: spaabi.RegisteredProof_StackedDRG32GiBPoSt,
			Candidates:      poStCandidates,
			Proofs:          []spaabi.PoStProof{{ProofBytes: proof}},
		},
	}
	surprisePostParams, err := abi.ToEncodedValues(surprisePoSt)
	if err != nil {
		return err
	}

	_, _, err = p.outbox.Send(
		ctx,
		minerState.Info.Worker,
		p.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		types.NewGasUnits(300),
		true,
		types.MethodID(builtin.MethodsMiner.SubmitSurprisePoStResponse),
		surprisePostParams,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *Poster) getProvingSet(ctx context.Context, minerState miner.State) (sectorbuilder.SortedPublicSectorInfo, error) {
	store := &actorStore{ctx, *p.chain}

	provingSet := adt.AsArray(store, minerState.ProvingSet)
	var sectorInfos []ffi.PublicSectorInfo
	var sector miner.SectorOnChainInfo
	err := provingSet.ForEach(&sector, func(idx int64) error {
		commRbytes, err := commcid.CIDToReplicaCommitmentV1(sector.Info.SealedCID)
		if err != nil {
			return err
		}

		var commR [sectorbuilder.CommLen]byte
		copy(commR[:], commRbytes)

		sectorInfo := ffi.PublicSectorInfo{
			SectorID: uint64(sector.Info.SectorNumber),
			CommR:    commR,
		}
		sectorInfos = append(sectorInfos, sectorInfo)
		return nil
	})
	if err != nil {
		return sectorbuilder.SortedPublicSectorInfo{}, err
	}

	return ffi.NewSortedPublicSectorInfo(sectorInfos...), nil
}

func (p *Poster) getFaults(ctx context.Context, minerState miner.State) ([]uint64, error) {
	store := &actorStore{ctx, *p.chain}

	// compute size of sectors to determine max faults (this may be overkill but it avoids a consensus parameter)
	var sector miner.SectorOnChainInfo
	sectorCount := uint64(0)
	err := adt.AsArray(store, minerState.Sectors).ForEach(&sector, func(idx int64) error {
		sectorCount++
		return nil
	})
	if err != nil {
		return nil, err
	}

	return minerState.FaultSet.All(sectorCount)
}

func (p *Poster) getChallengeSeed(ctx context.Context, newHead block.TipSet) ([32]byte, error) {
	var challengeSeed [32]byte

	height, err := newHead.Height()
	if err != nil {
		return challengeSeed, err
	}

	randomness, err := p.chain.SampleRandomness(ctx, types.NewBlockHeight(height))
	if err != nil {
		return challengeSeed, err
	}

	copy(challengeSeed[:], randomness)

	return challengeSeed, nil
}

func (p *Poster) getMinerID(ctx context.Context, newHead block.TipSet) (uint64, error) {
	addr, err := p.chain.ResolveAddressAt(ctx, newHead.Key(), p.minerAddr)
	if err != nil {
		return 0, err
	}

	return address.IDFromAddress(addr)
}

func (p *Poster) cancelPost() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.postCancel != nil {
		p.postCancel()
		p.postCancel = nil
	}
}
