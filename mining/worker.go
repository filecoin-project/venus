package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmSkuaNgyGmV8c1L3cZNWcUxRJV6J3nsD96JVQPcWcwtyW/go-hamt-ipld"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"
)

var (
	ticketDomain *big.Int
	// mineWarnThreshold is the number of seconds of mining after which the worker
	// logs a warning that mining is wasting an unexpected amount of work.
	mineWarnThreshold float64
	log               = logging.Logger("mining")
)

// mineSleepTime is the estimated mining time.  We define this so that we can
// fake mining with the current incomplete system.  TODO this needs to be
// configurable to expediate both unit and large scale testing.
const mineSleepTime = mineDelay * 30

func init() {
	ticketDomain = &big.Int{}
	ticketDomain.Exp(big.NewInt(2), big.NewInt(256), nil)
	ticketDomain.Sub(ticketDomain, big.NewInt(1))

	mineWarnThreshold = mineSleepTime.Seconds() / 2.0
}

// Input is the TipSets the worker should mine on, the address
// to accrue rewards to, and a context that the caller can use
// to cancel this mining run.
type Input struct {
	TipSet core.TipSet
}

// NewInput instantiates a new Input.
func NewInput(ts core.TipSet) Input {
	return Input{TipSet: ts}
}

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
	Mine(runCtx context.Context, input Input, outCh chan<- Output)
}

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, core.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, core.TipSet) (uint64, uint64, error)

type miningApplier func(ctx context.Context, messages []*types.SignedMessage, st state.Tree, vms vm.StorageMap, bh *types.BlockHeight) (core.ApplyMessagesResponse, error)

// DefaultWorker runs a mining job.
type DefaultWorker struct {
	createPoST DoSomeWorkFunc // TODO: rename createPoSTFunc
	minerAddr  types.Address  // TODO: needs to be a key in the near future

	// consensus things
	getStateTree GetStateTree
	getWeight    GetWeight

	// core filecoin things
	messagePool   *core.MessagePool
	applyMessages miningApplier
	powerTable    core.PowerTableView
	blockstore    blockstore.Blockstore
	cstore        *hamt.CborIpldStore
}

// NewDefaultWorker instantiates a new Worker.
func NewDefaultWorker(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable core.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner types.Address) *DefaultWorker {
	return NewDefaultWorkerWithDeps(messagePool, getStateTree, getWeight, applyMessages, powerTable, bs, cst, miner, createPoST)
}

// NewDefaultWorkerWithDeps instantiates a new Worker with custom functions.
func NewDefaultWorkerWithDeps(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable core.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner types.Address, createPoST DoSomeWorkFunc) *DefaultWorker {
	return &DefaultWorker{
		getStateTree:  getStateTree,
		getWeight:     getWeight,
		messagePool:   messagePool,
		applyMessages: applyMessages,
		powerTable:    powerTable,
		blockstore:    bs,
		cstore:        cst,
		createPoST:    createPoST,
		minerAddr:     miner,
	}
}

// DoSomeWorkFunc is a dummy function that mimics doing something time-consuming
// in the mining loop such as computing proofs. Pass a function that calls Sleep()
// is a good idea for now.
type DoSomeWorkFunc func()

// Mine implements the DefaultWorkers main mining function..
func (w *DefaultWorker) Mine(ctx context.Context, input Input, outCh chan<- Output) {
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)
	if len(input.TipSet) == 0 {
		outCh <- Output{Err: errors.New("bad input tipset with no blocks sent to Mine()")}
		return
	}
	// TODO: derive these from actual storage power.
	// This should now be pretty easy because the worker has getState and
	// powertable view.
	// To fix this and keep mock-mine mode actually generating blocks we'll
	// need to update the view to give every miner a little power in the
	// network.
	const myPower = 1
	const totalPower = 5

	for nullBlkCount := uint64(0); ; nullBlkCount++ {
		log.Infof("Mining on tipset: %s, with %d null blocks.", input.TipSet.String(), nullBlkCount)
		start := time.Now()
		if ctx.Err() != nil {
			return
		}

		challenge := createChallenge(input.TipSet, nullBlkCount)
		prCh := createProof(challenge, w.createPoST)
		var ticket []byte
		select {
		case <-ctx.Done():
			mineTime := time.Since(start)
			log.Infof("Mining run on: %s canceled.", input.TipSet.String())
			if mineTime.Seconds() < mineWarnThreshold {
				log.Warningf("Abandoning mining after %f seconds.  Wasting lots of work...", mineTime.Seconds())
			}
			return
		case proof := <-prCh:
			ticket = createTicket(proof)
		}

		// TODO: Test the interplay of isWinningTicket() and createPoST()
		if isWinningTicket(ticket, myPower, totalPower) {
			next, err := w.Generate(ctx, input.TipSet, ticket, nullBlkCount)
			if err == nil {
				log.SetTag(ctx, "block", next)
			}
			outCh <- NewOutput(next, err)
		}
	}
}

// TODO -- in general this won't work with only the base tipset, we'll potentially
// need some chain manager utils, similar to the State function, to sample
// further back in the chain.
func createChallenge(parents core.TipSet, nullBlkCount uint64) []byte {
	// Find the smallest ticket from parent set
	var smallest types.Signature
	for _, v := range parents {
		if smallest == nil || bytes.Compare(v.Ticket, smallest) < 0 {
			smallest = v.Ticket
		}
	}

	buf := make([]byte, 4)
	n := binary.PutUvarint(buf, nullBlkCount)
	buf = append(smallest, buf[:n]...)

	h := sha256.Sum256(buf)
	return h[:]
}

// TODO: Actually use the results of the PoST once it is implemented.
// Currently createProof just passes the challenge value through.
func createProof(challenge []byte, createPoST DoSomeWorkFunc) <-chan []byte {
	c := make(chan []byte)
	go func() {
		createPoST() // TODO send new PoST on channel once we can create it
		c <- challenge
	}()
	return c
}

func createTicket(proof []byte) []byte {
	h := sha256.Sum256(proof)
	// TODO: sign h once we have keys.
	return h[:]
}

var isWinningTicket = func(ticket []byte, myPower, totalPower int64) bool {
	// See https://github.com/filecoin-project/aq/issues/70 for an explanation of the math here.
	lhs := &big.Int{}
	lhs.SetBytes(ticket)
	lhs.Mul(lhs, big.NewInt(totalPower))

	rhs := &big.Int{}
	rhs.Mul(big.NewInt(myPower), ticketDomain)

	return lhs.Cmp(rhs) < 0
}

// createPoST is the default implementation of DoSomeWorkFunc. Contrary to the
// advertisement, it doesn't do anything yet.
func createPoST() {
	time.Sleep(mineSleepTime)
}
