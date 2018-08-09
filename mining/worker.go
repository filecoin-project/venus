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
	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
	"gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	logging "gx/ipfs/QmcVVHfdyv15GVPk7NrxdWjh2hLVccXnoD8j2tyQShiXJb/go-log"
)

var (
	ticketDomain *big.Int
	log          = logging.Logger("mining")
)

func init() {
	ticketDomain = &big.Int{}
	ticketDomain.Exp(big.NewInt(2), big.NewInt(256), nil)
	ticketDomain.Sub(ticketDomain, big.NewInt(1))
}

// Input is the TipSets the worker should mine on, the address
// to accrue rewards to, and a context that the caller can use
// to cancel this mining run.
type Input struct {
	Ctx      context.Context // TODO: we should evaluate if this is still useful
	TipSet   core.TipSet
	NullBlks uint64
}

// NewInput instantiates a new Input.
func NewInput(ctx context.Context, ts core.TipSet) Input {
	return Input{Ctx: ctx, TipSet: ts}
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

// MiningWorker runs a mining job.
type MiningWorker struct {
	createPoST DoSomeWorkFunc // TODO: rename createPoSTFunc?
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

// NewMiningWorker instantiates a new Worker.
func NewMiningWorker(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable core.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner types.Address) *MiningWorker {
	return NewMiningWorkerWithDeps(messagePool, getStateTree, getWeight, applyMessages, powerTable, bs, cst, miner, createPoST)
}

// NewMiningWorkerWithDeps instantiates a new Worker with custom functions.
func NewMiningWorkerWithDeps(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable core.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner types.Address, createPoST DoSomeWorkFunc) *MiningWorker {
	return &MiningWorker{
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

// Mine implements the MiningWorkers main mining function..
func (w *MiningWorker) Mine(ctx context.Context, input Input, outCh chan<- Output) {
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)

	// TODO: derive these from actual storage power.
	// This should now be pretty easy because the worker has getState and
	// powertable view.
	// To fix this and keep mock-mine mode actually generating blocks we'll
	// need to update the view to give every miner a little power in the
	// network.
	const myPower = 1
	const totalPower = 5

	if ctx.Err() != nil {
		return
	}

	challenge := createChallenge(input.TipSet, input.NullBlks)
	proof := createProof(challenge, w.createPoST)
	ticket := createTicket(proof)

	// TODO: Test the interplay of isWinningTicket() and createPoST()
	if isWinningTicket(ticket, myPower, totalPower) {
		next, err := w.Generate(ctx, input.TipSet, ticket, input.NullBlks)
		if err == nil {
			log.SetTag(ctx, "block", next)
		}

		if ctx.Err() == nil {
			outCh <- NewOutput(next, err)
		} else {
			log.Warningf("Abandoning successfully mined block without publishing: %s", input.TipSet.String())
		}
	}
	time.Sleep(mineSleepTime)
}

func createChallenge(parents core.TipSet, nullBlockCount uint64) []byte {
	// Find the smallest ticket from parent set
	var smallest types.Signature
	for _, v := range parents {
		if smallest == nil || bytes.Compare(v.Ticket, smallest) < 0 {
			smallest = v.Ticket
		}
	}

	buf := make([]byte, 4)
	n := binary.PutUvarint(buf, nullBlockCount)
	buf = append(smallest, buf[:n]...)

	h := sha256.Sum256(buf)
	return h[:]
}

func createProof(challenge []byte, createPoST DoSomeWorkFunc) []byte {
	// TODO: Actually use the results of the PoST once it is implemented.
	createPoST()
	return challenge
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
