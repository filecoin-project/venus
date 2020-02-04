package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"

	ffi "github.com/filecoin-project/filecoin-ffi"

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
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
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
type GetStateTree func(context.Context, block.TipSetKey) (state.Tree, error)

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
	// Dragons: add something back or remove
}

type workerPorcelainAPI interface {
	BlockTime() time.Duration
	MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)
	Snapshot(ctx context.Context, baseKey block.TipSetKey) (consensus.ActorStateSnapshot, error)
}

type electionUtil interface {
	GeneratePoStRandomness(block.Ticket, address.Address, types.Signer, uint64) ([]byte, error)
	GenerateCandidates([]byte, ffi.SortedPublicSectorInfo, postgenerator.PoStGenerator) ([]ffi.Candidate, error)
	GeneratePoSt(ffi.SortedPublicSectorInfo, []byte, []ffi.Candidate, postgenerator.PoStGenerator) ([]byte, error)
	CandidateWins([]byte, uint64, uint64, uint64, uint64) bool
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

	tsMetadata    tipSetMetadata
	getStateTree  GetStateTree
	getWeight     GetWeight
	getAncestors  GetAncestors
	election      electionUtil
	ticketGen     ticketGenerator
	messageSource MessageSource
	processor     MessageApplier
	messageStore  chain.MessageWriter // nolint: structcheck
	blockstore    blockstore.Blockstore
	clock         clock.Clock
	poster        postgenerator.PoStGenerator
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
	Poster        postgenerator.PoStGenerator
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
		poster:         parameters.Poster,
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
	postRandomness, err := w.election.GeneratePoStRandomness(electionTicket, workerAddr, w.workerSigner, nullBlkCount)
	if err != nil {
		log.Errorf("Worker.Mine failed to generate post randomness %s", err)
		outCh <- Output{Err: err}
		return
	}
	powerTable, err := w.getPowerTable(ctx, base.Key())
	if err != nil {
		log.Errorf("Worker.Mine couldn't get snapshot for tipset: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}
	sortedSectorInfos, err := powerTable.SortedSectorInfos(ctx, w.minerAddr)
	if err != nil {
		log.Warnf("Worker.Mine failed to get ssi for %s", w.minerAddr.String())
		outCh <- Output{Err: err}
		return
	}
	// Generate election post candidates
	done := make(chan []ffi.Candidate)
	errCh := make(chan error)
	go func() {
		defer close(done)
		defer close(errCh)
		candidates, err := w.election.GenerateCandidates(postRandomness, sortedSectorInfos, w.poster)
		if err != nil {
			errCh <- err
			return
		}
		done <- candidates
	}()
	var candidates []ffi.Candidate
	select {
	case <-ctx.Done():
		log.Infow("Mining run on tipset with null blocks canceled.", "tipset", base, "nullBlocks", nullBlkCount)
	case err := <-errCh:
		log.Warnf("Worker.Mine failed to get ssi for %s", err)
		outCh <- Output{Err: err}
		return
	case genResult := <-done:
		candidates = genResult
	}

	// Look for any winning candidates
	sectorNum, err := powerTable.NumSectors(ctx, w.minerAddr)
	if err != nil {
		log.Errorf("failed to get number of sectors for miner: %s", err)
		outCh <- Output{Err: err}
		return
	}
	networkPower, err := powerTable.Total(ctx)
	if err != nil {
		log.Errorf("failed to get total power: %s", err)
		outCh <- Output{Err: err}
		return
	}
	sectorSize, err := powerTable.SectorSize(ctx, w.minerAddr)
	if err != nil {
		log.Errorf("failed to get sector size for miner: %s", err)
		outCh <- Output{Err: err}
		return
	}
	hasher := hasher.NewHasher()
	var winners []ffi.Candidate
	for _, candidate := range candidates {
		hasher.Bytes(candidate.PartialTicket[:])
		challengeTicket := hasher.Hash()
		if w.election.CandidateWins(challengeTicket, sectorNum, 0, networkPower.Uint64(), sectorSize.Uint64()) {
			winners = append(winners, candidate)
		}
	}

	// no winners we are done
	if len(winners) == 0 {
		return
	}
	// we have a winning block

	// Generate PoSt
	postDone := make(chan []byte)
	errCh = make(chan error)
	go func() {
		defer close(postDone)
		defer close(errCh)
		post, err := w.election.GeneratePoSt(sortedSectorInfos, postRandomness, winners, w.poster)
		if err != nil {
			errCh <- err
			return
		}
		postDone <- post
	}()
	var post []byte
	select {
	case <-ctx.Done():
		log.Infow("Mining run on tipset with null blocks canceled.", "tipset", base, "nullBlocks", nullBlkCount)
	case err := <-errCh:
		log.Warnf("Worker.Mine failed to generate post %s", err)
		outCh <- Output{Err: err}
		return
	case postOut := <-postDone:
		post = postOut
	}

	postInfo := block.NewEPoStInfo(post, postRandomness, block.FromFFICandidates(winners...)...)

	next, err := w.Generate(ctx, base, nextTicket, nullBlkCount, postInfo)
	if err == nil {
		log.Debugf("Worker.Mine generates new winning block! %s", next.Cid().String())
	}
	outCh <- NewOutput(next, err)
	won = true
	return
}

func (w *DefaultWorker) getPowerTable(ctx context.Context, baseKey block.TipSetKey) (consensus.PowerTableView, error) {
	snapshot, err := w.api.Snapshot(ctx, baseKey)
	if err != nil {
		return consensus.PowerTableView{}, err
	}
	return consensus.NewPowerTableView(snapshot), nil
}
