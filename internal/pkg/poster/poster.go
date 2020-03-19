package poster

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-sectorbuilder"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/prometheus/common/log"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// Poster listens for changes to the chain head and generates and submits a PoSt if one is required.
type Poster struct {
	postMutex      sync.Mutex
	postCancel     context.CancelFunc
	scheduleCancel context.CancelFunc

	minerAddr     address.Address
	outbox        *message.Outbox
	sectorbuilder sectorbuilder.Interface
	chain         *cst.ChainStateReadWriter
	stateViewer   *appstate.Viewer
	waiter        *msg.Waiter
}

// NewPoster creates a Poster struct
func NewPoster(
	minerAddr address.Address,
	outbox *message.Outbox,
	sb sectorbuilder.Interface,
	chain *cst.ChainStateReadWriter,
	stateViewer *appstate.Viewer,
	waiter *msg.Waiter) *Poster {

	return &Poster{
		minerAddr:     minerAddr,
		outbox:        outbox,
		sectorbuilder: sb,
		chain:         chain,
		stateViewer:   stateViewer,
		waiter:        waiter,
	}
}

// HandleNewHead submits a new chain head for possible fallback PoSt.
func (p *Poster) HandleNewHead(ctx context.Context, newHead block.TipSet) error {
	return p.startPoStIfNeeded(ctx, newHead)
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

	root, err := p.chain.GetTipSetStateRoot(ctx, newHead.Key())
	if err != nil {
		return err
	}

	stateView := p.stateViewer.StateView(root)
	provingPeriodStart, _, _, err := stateView.MinerProvingPeriod(ctx, p.minerAddr)
	if err != nil {
		return nil
	}

	if provingPeriodStart > tipsetHeight {
		// it's not time to PoSt
		return nil
	}

	ctx, p.postCancel = context.WithCancel(ctx)
	go p.doPoSt(ctx, stateView, provingPeriodStart, newHead.Key())

	return nil
}

func (p *Poster) doPoSt(ctx context.Context, stateView *appstate.View, provingPeriodStart abi.ChainEpoch, head block.TipSetKey) {
	defer p.cancelPost()

	sortedSectorInfo, err := p.getProvingSet(ctx, stateView)
	if err != nil {
		log.Error("error getting proving set: ", err)
		return
	}

	faults, err := stateView.MinerFaults(ctx, p.minerAddr)
	if err != nil {
		log.Error("error getting faults: ", err)
		return
	}

	challengeSeed, err := p.getChallengeSeed(ctx, head, provingPeriodStart, p.minerAddr)
	if err != nil {
		log.Error("error getting challenge seed: ", err)
		return
	}

	faultsPrime := make([]abi.SectorNumber, len(faults))
	for idx := range faultsPrime {
		faultsPrime[idx] = abi.SectorNumber(faults[idx])
	}

	candidates, proofs, err := p.sectorbuilder.GenerateFallbackPoSt(sortedSectorInfo, abi.PoStRandomness(challengeSeed[:]), faultsPrime)
	if err != nil {
		log.Error("error generating fallback PoSt: ", err)
		return
	}

	poStCandidates := make([]abi.PoStCandidate, len(candidates))
	for i := range candidates {
		poStCandidates[i] = candidates[i].Candidate
	}

	err = p.sendPoSt(ctx, stateView, poStCandidates, proofs)
	if err != nil {
		log.Error("error sending fallback PoSt: ", err)
		return
	}
}

func (p *Poster) sendPoSt(ctx context.Context, stateView *appstate.View, candidates []abi.PoStCandidate, proofs []abi.PoStProof) error {

	windowedPost := &abi.OnChainPoStVerifyInfo{
		Candidates: candidates,
		Proofs:     proofs,
	}

	_, workerAddr, err := stateView.MinerControlAddresses(ctx, p.minerAddr)
	if err != nil {
		return err
	}

	signerAddr, err := stateView.AccountSignerAddress(ctx, workerAddr)
	if err != nil {
		return err
	}

	mcid, _, err := p.outbox.Send(
		ctx,
		signerAddr,
		p.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		gas.NewGas(5000),
		true,
		builtin.MethodsMiner.SubmitWindowedPoSt,
		windowedPost,
	)
	if err != nil {
		return err
	}

	// wait until we see the post on chain at least once
	err = p.waiter.Wait(ctx, mcid, func(_ *block.Block, _ *types.SignedMessage, recp *vm.MessageReceipt) error {
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Poster) getProvingSet(ctx context.Context, stateView *appstate.View) ([]abi.SectorInfo, error) {
	return consensus.NewPowerTableView(stateView).SortedSectorInfos(ctx, p.minerAddr)
}

func (p *Poster) getChallengeSeed(ctx context.Context, head block.TipSetKey, height abi.ChainEpoch, minerAddr address.Address) ([32]byte, error) {
	var challengeSeed [32]byte

	entropy, err := encoding.Encode(minerAddr)
	if err != nil {
		return challengeSeed, err
	}
	randomness, err := p.chain.SampleChainRandomness(ctx, head, acrypto.DomainSeparationTag_WindowedPoStChallengeSeed, height, entropy)
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
