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
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
)

var log = logging.Logger("poster")

// epochs to wait past proving period to avoid invalid posts due to reorgs
var confidenceInterval uint64 = 7

// Poster listens for changes to the chain head and generates and submits a PoSt if one is required.
type Poster struct {
	postMutex         sync.Mutex
	postCancel        context.CancelFunc
	scheduleCancel    context.CancelFunc
	poStDeadlineIndex int64

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
		minerAddr:         minerAddr,
		outbox:            outbox,
		mgr:               mgr,
		chain:             chain,
		stateViewer:       stateViewer,
		waiter:            waiter,
		poStDeadlineIndex: -1,
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
	index, _, _, challenge, err := stateView.MinerDeadlineInfo(ctx, p.minerAddr, tipsetHeight)
	if err != nil {
		return nil
	}

	// check that we haven't already seen this deadline
	if int64(index) == p.poStDeadlineIndex {
		return nil
	}
	p.poStDeadlineIndex = int64(index)

	ctx, p.postCancel = context.WithCancel(ctx)
	go p.doPoSt(ctx, stateView, index, challenge, newHead.Key())

	return nil
}

func (p *Poster) doPoSt(ctx context.Context, stateView *appstate.View, deadlineIndex uint64, challengeAt abi.ChainEpoch, head block.TipSetKey) {
	defer p.cancelPost()

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

	// if no sectors, we're done
	if len(sectors) == 0 {
		return
	}

	buf := new(bytes.Buffer)
	err = p.minerAddr.MarshalCBOR(buf)
	if err != nil {
		log.Errorf("could not create entropy from miner address %s: %s", p.minerAddr, err)
		return
	}

	randomness, err := p.chain.SampleChainRandomness(ctx, head, acrypto.DomainSeparationTag_WindowedPoStChallengeSeed, challengeAt, buf.Bytes())
	if err != nil {
		log.Errorf("could not sample chain randomness at %d: %s", challengeAt, err)
		return
	}

	proofs, err := p.mgr.GenerateWindowPoSt(ctx, abi.ActorID(minerID), sectors, abi.PoStRandomness(randomness))
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
	err = p.waiter.Wait(ctx, mcid, func(_ *block.Block, _ *types.SignedMessage, recp *vm.MessageReceipt) error {
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Poster) getProvingSet(ctx context.Context, stateView *appstate.View) ([]abi.SectorInfo, error) {
	return consensus.NewPowerTableView(stateView, stateView).SortedSectorInfos(ctx, p.minerAddr)
}

func (p *Poster) cancelPost() {
	p.postMutex.Lock()
	defer p.postMutex.Unlock()

	if p.postCancel != nil {
		p.postCancel()
		p.postCancel = nil
	}
}
