package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var log = logging.Logger("mining")

// Output is the result of a single mining run. It has either a new
// block or an error, mimicing the golang (retVal, error) pattern.
// If a mining run's context is canceled there is no output.
type Output struct {
	NewBlock *block.Block
	Err      error
}

// NewOutput instantiates a new Output.
func NewOutput(b *block.Block, e error) Output {
	return Output{NewBlock: b, Err: e}
}

// Worker is the interface called by the Scheduler to run the mining work being
// scheduled.
type Worker interface {
	Mine(runCtx context.Context, base block.TipSet, nullBlkCount uint64, outCh chan<- Output) bool
}

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, block.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, block.TipSet) (uint64, error)

// GetAncestors is a function that returns the necessary ancestor chain to
// process the input tipset.
type GetAncestors func(context.Context, block.TipSet, *types.BlockHeight) ([]block.TipSet, error)

// MessageSource provides message candidates for mining into blocks
type MessageSource interface {
	// Pending returns a slice of un-mined messages.
	Pending() []*types.SignedMessage
	// Remove removes a message from the source permanently
	Remove(message cid.Cid)
}

// A MessageApplier processes all the messages in a message pool.
type MessageApplier interface {
	// ApplyMessagesAndPayRewards applies all state transitions related to a set of messages.
	ApplyMessagesAndPayRewards(ctx context.Context, st state.Tree, vms vm.StorageMap, messages []*types.UnsignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight, ancestors []block.TipSet) ([]*consensus.ApplyMessageResult, error)
}

type workerPorcelainAPI interface {
	BlockTime() time.Duration
	MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)
	Snapshot(ctx context.Context, baseKey block.TipSetKey) (consensus.ActorStateSnapshot, error)
}

type electionUtil interface {
	RunElection(block.Ticket, address.Address, types.Signer, uint64) (block.VRFPi, error)
	IsElectionWinner(context.Context, consensus.PowerTableView, block.Ticket, uint64, block.VRFPi, address.Address, address.Address) (bool, error)
}

// ticketGenerator creates tickets.
type ticketGenerator interface {
	NextTicket(block.Ticket, address.Address, types.Signer) (block.Ticket, error)
}

type tipSetMetadata interface {
	GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error)
	GetTipSetReceiptsRoot(key block.TipSetKey) (cid.Cid, error)
}

// DefaultWorker runs a mining job.
type DefaultWorker struct {
	api workerPorcelainAPI

	minerAddr      address.Address
	minerOwnerAddr address.Address
	workerSigner   types.Signer

	// consensus things
	tsMetadata   tipSetMetadata
	getStateTree GetStateTree
	getWeight    GetWeight
	getAncestors GetAncestors
	election     electionUtil
	ticketGen    ticketGenerator

	// core filecoin things
	messageSource MessageSource
	processor     MessageApplier
	messageStore  chain.MessageWriter // nolint: structcheck
	blockstore    blockstore.Blockstore
	clock         clock.Clock
}

// WorkerParameters use for NewDefaultWorker parameters
type WorkerParameters struct {
	API workerPorcelainAPI

	MinerAddr      address.Address
	MinerOwnerAddr address.Address
	WorkerSigner   types.Signer

	// consensus things
	TipSetMetadata tipSetMetadata
	GetStateTree   GetStateTree
	GetWeight      GetWeight
	GetAncestors   GetAncestors
	Election       electionUtil
	TicketGen      ticketGenerator

	// core filecoin things
	MessageSource MessageSource
	Processor     MessageApplier
	MessageStore  chain.MessageWriter
	Blockstore    blockstore.Blockstore
	Clock         clock.Clock
}

// NewDefaultWorker instantiates a new Worker.
func NewDefaultWorker(parameters WorkerParameters) *DefaultWorker {
	return &DefaultWorker{
		api:            parameters.API,
		getStateTree:   parameters.GetStateTree,
		getWeight:      parameters.GetWeight,
		getAncestors:   parameters.GetAncestors,
		messageSource:  parameters.MessageSource,
		messageStore:   parameters.MessageStore,
		processor:      parameters.Processor,
		blockstore:     parameters.Blockstore,
		minerAddr:      parameters.MinerAddr,
		minerOwnerAddr: parameters.MinerOwnerAddr,
		workerSigner:   parameters.WorkerSigner,
		election:       parameters.Election,
		ticketGen:      parameters.TicketGen,
		tsMetadata:     parameters.TipSetMetadata,
		clock:          parameters.Clock,
	}
}

// Mine implements the DefaultWorkers main mining function..
// The returned bool indicates if this miner created a new block or not.
func (w *DefaultWorker) Mine(ctx context.Context, base block.TipSet, nullBlkCount uint64, outCh chan<- Output) (won bool) {
	log.Info("Worker.Mine")
	if !base.Defined() {
		log.Warn("Worker.Mine returning because it can't mine on an empty tipset")
		outCh <- Output{Err: errors.New("bad input tipset with no blocks sent to Mine()")}
		return
	}

	log.Debugf("Mining on tipset: %s, with %d null blocks.", base.String(), nullBlkCount)
	if ctx.Err() != nil {
		log.Warnf("Worker.Mine returning with ctx error %s", ctx.Err().Error())
		return
	}

	// Read uncached worker address
	workerAddr, err := w.api.MinerGetWorkerAddress(ctx, w.minerAddr, base.Key())
	if err != nil {
		outCh <- Output{Err: err}
		return
	}

	// lookback consensus.ElectionLookback
	prevTicket, err := base.MinTicket()
	if err != nil {
		log.Warnf("Worker.Mine couldn't read parent ticket %s", err)
		outCh <- Output{Err: err}
		return
	}

	nextTicket, err := w.ticketGen.NextTicket(prevTicket, workerAddr, w.workerSigner)
	if err != nil {
		log.Warnf("Worker.Mine couldn't generate next ticket %s", err)
		outCh <- Output{Err: err}
		return
	}

	// Provably delay for the blocktime
	done := make(chan struct{})
	go func() {
		defer close(done)
		// TODO #2223 remove this explicit wait if/when NotarizeTime calls VDF
		w.clock.Sleep(w.api.BlockTime())
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		log.Infow("Mining run on tipset with null blocks canceled.", "tipset", base, "nullBlocks", nullBlkCount)
		return
	case <-done:
	}

	// lookback consensus.ElectionLookback for the election ticket
	baseHeight, err := base.Height()
	if err != nil {
		log.Warnf("Worker.Mine couldn't read base height %s", err)
		outCh <- Output{Err: err}
		return
	}
	ancestors, err := w.getAncestors(ctx, base, types.NewBlockHeight(baseHeight+nullBlkCount+1))
	if err != nil {
		log.Warnf("Worker.Mine couldn't get ancestorst %s", err)
		outCh <- Output{Err: err}
		return
	}
	electionTicket, err := sampling.SampleNthTicket(consensus.ElectionLookback-1, ancestors)
	if err != nil {
		log.Warnf("Worker.Mine couldn't read parent ticket %s", err)
		outCh <- Output{Err: err}
		return
	}

	// Run an election to check if this miner has won the right to mine
	electionProof, err := w.election.RunElection(electionTicket, workerAddr, w.workerSigner, nullBlkCount)
	if err != nil {
		log.Errorf("failed to run local election: %s", err)
		outCh <- Output{Err: err}
		return
	}
	powerTable, err := w.getPowerTable(ctx, base.Key())
	if err != nil {
		log.Errorf("Worker.Mine couldn't get snapshot for tipset: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}
	weHaveAWinner, err := w.election.IsElectionWinner(ctx, powerTable, electionTicket, nullBlkCount, electionProof, workerAddr, w.minerAddr)
	if err != nil {
		log.Errorf("Worker.Mine couldn't run election: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}

	// This address has mining rights, so mine a block
	if weHaveAWinner {
		next, err := w.Generate(ctx, base, nextTicket, electionProof, nullBlkCount)
		if err == nil {
			log.Debugf("Worker.Mine generates new winning block! %s", next.Cid().String())
		}
		outCh <- NewOutput(next, err)
		won = true
		return
	}
	return
}

func (w *DefaultWorker) getPowerTable(ctx context.Context, baseKey block.TipSetKey) (consensus.PowerTableView, error) {
	snapshot, err := w.api.Snapshot(ctx, baseKey)
	if err != nil {
		return consensus.PowerTableView{}, err
	}
	return consensus.NewPowerTableView(snapshot), nil
}
