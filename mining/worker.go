package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

var log = logging.Logger("mining")

// Output is the result of a single mining run. It has either a new
// block or an error, mimicing the golang (retVal, error) pattern.
// If a mining run's context is canceled there is no output.
type Output struct {
	NewBlock *types.Block
	Err      error
}

// NewOutput instantiates a new Output.
func NewOutput(b *types.Block, e error) Output {
	return Output{NewBlock: b, Err: e}
}

// Worker is the interface called by the Scheduler to run the mining work being
// scheduled.
type Worker interface {
	Mine(runCtx context.Context, base types.TipSet, ticketArray []types.Ticket, outCh chan<- Output) (bool, types.Ticket)
}

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, types.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, types.TipSet) (uint64, error)

// GetAncestors is a function that returns the necessary ancestor chain to
// process the input tipset.
type GetAncestors func(context.Context, types.TipSet, *types.BlockHeight) ([]types.TipSet, error)

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
	ApplyMessagesAndPayRewards(ctx context.Context, st state.Tree, vms vm.StorageMap, messages []*types.SignedMessage, minerOwnerAddr address.Address, bh *types.BlockHeight, ancestors []types.TipSet) (consensus.ApplyMessagesResponse, error)
}

type workerPorcelainAPI interface {
	BlockTime() time.Duration
	MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey types.TipSetKey) (address.Address, error)
	Queryer(ctx context.Context, baseKey types.TipSetKey) (consensus.ActorStateSnapshot, error)
}

type electionUtil interface {
	RunElection(types.Ticket, address.Address, types.Signer) (types.VRFPi, error)
	IsElectionWinner(context.Context, consensus.PowerTableView, types.Ticket, types.VRFPi, address.Address, address.Address) (bool, error)
}

// ticketGenerator creates and finalizes tickets.
type ticketGenerator interface {
	NextTicket(types.Ticket, address.Address, types.Signer) (types.Ticket, error)
	NotarizeTime(*types.Ticket) error
}

// DefaultWorker runs a mining job.
type DefaultWorker struct {
	api workerPorcelainAPI

	minerAddr      address.Address
	minerOwnerAddr address.Address
	workerSigner   types.Signer

	// consensus things
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
	GetStateTree GetStateTree
	GetWeight    GetWeight
	GetAncestors GetAncestors
	Election     electionUtil
	TicketGen    ticketGenerator

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
		clock:          parameters.Clock,
	}
}

// Mine implements the DefaultWorkers main mining function..
// The returned bool indicates if this miner created a new block or not.
func (w *DefaultWorker) Mine(ctx context.Context, base types.TipSet, ticketArray []types.Ticket, outCh chan<- Output) (won bool, nextTicket types.Ticket) {
	log.Info("Worker.Mine")
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)
	if !base.Defined() {
		log.Warning("Worker.Mine returning because it can't mine on an empty tipset")
		outCh <- Output{Err: errors.New("bad input tipset with no blocks sent to Mine()")}
		return
	}

	log.Debugf("Mining on tipset: %s, with %d null blocks.", base.String(), len(ticketArray))
	if ctx.Err() != nil {
		log.Warningf("Worker.Mine returning with ctx error %s", ctx.Err().Error())
		return
	}

	// Read uncached worker address
	workerAddr, err := w.api.MinerGetWorkerAddress(ctx, w.minerAddr, base.Key())
	if err != nil {
		outCh <- Output{Err: err}
		return
	}

	// Create the next ticket.
	// With 0 null blocks derive from mining base's min ticket
	// With > 0 null blocks use the last mined ticket
	var prevTicket types.Ticket
	if len(ticketArray) == 0 {
		prevTicket, err = base.MinTicket()
		if err != nil {
			log.Warningf("Worker.Mine couldn't read parent ticket %s", err)
			outCh <- Output{Err: err}
			return
		}
	} else {
		prevTicket = ticketArray[len(ticketArray)-1]
	}

	nextTicket, err = w.ticketGen.NextTicket(prevTicket, workerAddr, w.workerSigner)
	if err != nil {
		log.Warningf("Worker.Mine couldn't generate next ticket %s", err)
		outCh <- Output{Err: err}
		return
	}

	// Provably delay for the blocktime
	done := make(chan struct{})
	errCh := make(chan error)
	go func() {
		defer close(done)
		defer close(errCh)
		err = w.ticketGen.NotarizeTime(&nextTicket)
		if err != nil {
			errCh <- err
		}
		// TODO #2223 remove this explicit wait if/when NotarizeTime calls VDF
		w.clock.Sleep(w.api.BlockTime())
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		log.Infof("Mining run on base %s with %d null blocks canceled.", base.String(), len(ticketArray))
		return
	case <-done:
	case err := <-errCh:
		log.Infof("Error notarizing time on ticket")
		outCh <- Output{Err: err}
		return
	}

	// Run an election to check if this miner has won the right to mine
	electionProof, err := w.election.RunElection(nextTicket, workerAddr, w.workerSigner)
	if err != nil {
		log.Errorf("failed to run local election: %s", err)
		outCh <- Output{Err: err}
		return
	}
	powerTable, err := w.getPowerTable(ctx, base.Key())
	if err != nil {
		log.Errorf("Worker.Mine couldn't get queryer for tipset: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}
	weHaveAWinner, err := w.election.IsElectionWinner(ctx, powerTable, nextTicket, electionProof, workerAddr, w.minerAddr)
	if err != nil {
		log.Errorf("Worker.Mine couldn't run election: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}

	// This address has mining rights, so mine a block
	if weHaveAWinner {
		next, err := w.Generate(ctx, base, append(ticketArray, nextTicket), electionProof, uint64(len(ticketArray)))
		if err == nil {
			log.SetTag(ctx, "block", next)
			log.Debugf("Worker.Mine generates new winning block! %s", next.Cid().String())
		}
		outCh <- NewOutput(next, err)
		won = true
		return
	}

	return
}

func (w *DefaultWorker) getPowerTable(ctx context.Context, baseKey types.TipSetKey) (consensus.PowerTableView, error) {
	queryer, err := w.api.Queryer(ctx, baseKey)
	if err != nil {
		return consensus.PowerTableView{}, err
	}
	return consensus.NewPowerTableView(queryer), nil
}
