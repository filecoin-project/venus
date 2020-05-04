package poster

import (
	"bytes"
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-address"
	sectorstorage "github.com/filecoin-project/sector-storage"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
)

var log = logging.Logger("poster")

// Poster listens for changes to the chain head and generates and submits a PoSt if one is required.
type Poster struct {
	postMutex      sync.Mutex
	postCancel     context.CancelFunc
	scheduleCancel context.CancelFunc
	challenge      abi.Randomness

	minerAddr   address.Address
	outbox      *message.Outbox
	mgr         sectorstorage.SectorManager
	chain       *cst.ChainStateReadWriter
	stateViewer *appstate.Viewer
	waiter      *msg.Waiter
}

// NewPoster creates a Poster struct
func NewPoster(
	minerAddr address.Address,
	outbox *message.Outbox,
	mgr sectorstorage.SectorManager,
	chain *cst.ChainStateReadWriter,
	stateViewer *appstate.Viewer,
	waiter *msg.Waiter) *Poster {

	return &Poster{
		minerAddr:   minerAddr,
		outbox:      outbox,
		mgr:         mgr,
		chain:       chain,
		stateViewer: stateViewer,
		waiter:      waiter,
		challenge:   abi.Randomness{},
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
	index, open, _, challengeAt, err := stateView.MinerDeadlineInfo(ctx, p.minerAddr, tipsetHeight)
	if err != nil {
		return err
	}

	// exit if we haven't yet hit the deadline
	if tipsetHeight < open {
		return nil
	}

	randomness, err := p.getChallenge(ctx, newHead.Key(), challengeAt)
	if err != nil {
		return err
	}

	// If we have not already seen this randomness, either the deadline has changed
	// or the chain as reorged to a point prior to the challenge. Either way,
	// it is time to start a new PoSt.
	if bytes.Equal(p.challenge, randomness) {
		return nil
	}
	p.challenge = randomness

	// stop existing PoSt, if one exists
	p.cancelPoSt()

	ctx, p.postCancel = context.WithCancel(ctx)
	go p.doPoSt(ctx, stateView, index)

	return nil
}

func (p *Poster) doPoSt(ctx context.Context, stateView *appstate.View, deadlineIndex uint64) {
	defer p.safeCancelPoSt()

	minerID, err := address.IDFromAddress(p.minerAddr)
	if err != nil {
		log.Errorf("Error retrieving miner ID from address %s: %s", p.minerAddr, err)
		return
	}

	partitions, err := stateView.MinerPartitionIndicesForDeadline(ctx, p.minerAddr, deadlineIndex)
	if err != nil {
		log.Errorf("Error retrieving partitions for address %s at index %d: %s", p.minerAddr, deadlineIndex, err)
		return
	}

	// if no partitions, we're done
	if len(partitions) == 0 {
		return
	}

	// Some day we might want to choose a subset of partitions to prove at one time. Today is not that day.
	sectors, err := stateView.MinerSectorInfoForDeadline(ctx, p.minerAddr, deadlineIndex, partitions)
	if err != nil {
		log.Errorf("error retrieving sector info for miner %s partitions at index %d: %s", p.minerAddr, deadlineIndex, err)
		return
	}

	proofs, err := p.mgr.GenerateWindowPoSt(ctx, abi.ActorID(minerID), sectors, abi.PoStRandomness(p.challenge))
	if err != nil {
		log.Errorf("error generating window PoSt: %s", err)
		return
	}

	_, workerAddr, err := stateView.MinerControlAddresses(ctx, p.minerAddr)
	if err != nil {
		log.Errorf("could not get miner worker address fro miner %s: %s", p.minerAddr, err)
		return
	}

	err = p.sendPoSt(ctx, workerAddr, deadlineIndex, partitions, proofs)
	if err != nil {
		log.Error("error sending window PoSt: ", err)
		return
	}
}

func (p *Poster) sendPoSt(ctx context.Context, workerAddr address.Address, index uint64, partitions []uint64, proofs []abi.PoStProof) error {

	windowedPost := &miner.SubmitWindowedPoStParams{
		Deadline:   index,
		Partitions: partitions,
		Proofs:     proofs,
		Skipped:    abi.BitField{},
	}

	mcid, errCh, err := p.outbox.Send(
		ctx,
		workerAddr,
		p.minerAddr,
		types.ZeroAttoFIL,
		types.NewGasPrice(1),
		gas.NewGas(10000),
		true,
		builtin.MethodsMiner.SubmitWindowedPoSt,
		windowedPost,
	)
	if err != nil {
		return err
	}
	if err := <-errCh; err != nil {
		return err
	}

	// wait until we see the post on chain at least once
	err = p.waiter.Wait(ctx, mcid, msg.DefaultMessageWaitLookback, func(_ *block.Block, _ *types.SignedMessage, recp *vm.MessageReceipt) error {
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Poster) getChallenge(ctx context.Context, head block.TipSetKey, at abi.ChainEpoch) (abi.Randomness, error) {
	buf := new(bytes.Buffer)
	err := p.minerAddr.MarshalCBOR(buf)
	if err != nil {
		return nil, err
	}

	return p.chain.SampleChainRandomness(ctx, head, acrypto.DomainSeparationTag_WindowedPoStChallengeSeed, at, buf.Bytes())
}

func (p *Poster) safeCancelPoSt() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	p.cancelPoSt()
}

func (p *Poster) cancelPoSt() {
	if p.postCancel != nil {
		p.postCancel()
		p.postCancel = nil
	}
}
