package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	sha256 "gx/ipfs/QmcTzQXRcU2vf8yX5EEboz1BSvWC7wWmeYAKVQmhp8WZYU/sha256-simd"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
)

var log = logging.Logger("mining")

// DefaultBlockTime is the estimated proving period time.
// We define this so that we can fake mining in the current incomplete system.
const DefaultBlockTime = 30 * time.Second

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
	Mine(runCtx context.Context, base consensus.TipSet, nullBlkCount int, outCh chan<- Output) bool
}

// GetStateTree is a function that gets the aggregate state tree of a TipSet. It's
// its own function to facilitate testing.
type GetStateTree func(context.Context, consensus.TipSet) (state.Tree, error)

// GetWeight is a function that calculates the weight of a TipSet.  Weight is
// expressed as two uint64s comprising a rational number.
type GetWeight func(context.Context, consensus.TipSet) (uint64, uint64, error)

type miningApplier func(ctx context.Context, messages []*types.SignedMessage, st state.Tree, vms vm.StorageMap, bh *types.BlockHeight) (consensus.ApplyMessagesResponse, error)

// DefaultWorker runs a mining job.
type DefaultWorker struct {
	createPoST DoSomeWorkFunc  // TODO: rename createPoSTFunc
	minerAddr  address.Address // TODO: needs to be a key in the near future

	// consensus things
	getStateTree GetStateTree
	getWeight    GetWeight

	// core filecoin things
	messagePool   *core.MessagePool
	applyMessages miningApplier
	powerTable    consensus.PowerTableView
	blockstore    blockstore.Blockstore
	cstore        *hamt.CborIpldStore
	blockTime     time.Duration
}

// NewDefaultWorker instantiates a new Worker.
func NewDefaultWorker(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable consensus.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner address.Address, bt time.Duration) *DefaultWorker {
	w := NewDefaultWorkerWithDeps(messagePool, getStateTree, getWeight, applyMessages, powerTable, bs, cst, miner, bt, func() {})
	w.createPoST = w.fakeCreatePoST
	return w
}

// NewDefaultWorkerWithDeps instantiates a new Worker with custom functions.
func NewDefaultWorkerWithDeps(messagePool *core.MessagePool, getStateTree GetStateTree, getWeight GetWeight, applyMessages miningApplier, powerTable consensus.PowerTableView, bs blockstore.Blockstore, cst *hamt.CborIpldStore, miner address.Address, bt time.Duration, createPoST DoSomeWorkFunc) *DefaultWorker {
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
		blockTime:     bt,
	}
}

// DoSomeWorkFunc is a dummy function that mimics doing something time-consuming
// in the mining loop such as computing proofs. Pass a function that calls Sleep()
// is a good idea for now.
type DoSomeWorkFunc func()

// Mine implements the DefaultWorkers main mining function..
// The returned bool indicates if this miner created a new block or not.
func (w *DefaultWorker) Mine(ctx context.Context, base consensus.TipSet, nullBlkCount int, outCh chan<- Output) bool {
	log.Info("Worker.Mine")
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)
	if len(base) == 0 {
		log.Warning("Worker.Mine returning because it can't mine on an empty tipset")
		outCh <- Output{Err: errors.New("bad input tipset with no blocks sent to Mine()")}
		return false
	}

	st, err := w.getStateTree(ctx, base)
	if err != nil {
		log.Errorf("Worker.Mine couldn't get state tree for tipset: %s", err.Error())
		outCh <- Output{Err: err}
		return false
	}

	log.Debugf("Mining on tipset: %s, with %d null blocks.", base.String(), nullBlkCount)
	if ctx.Err() != nil {
		log.Warningf("Worker.Mine returning with ctx error %s", ctx.Err().Error())
		return false
	}

	challenge, err := consensus.CreateChallenge(base, uint64(nullBlkCount))
	if err != nil {
		outCh <- Output{Err: err}
		return false
	}
	prCh := createProof(challenge, w.createPoST)
	var ticket []byte
	select {
	case <-ctx.Done():
		log.Infof("Mining run on base %s with %d null blocks canceled.", base.String(), nullBlkCount)
		return false
	case proof := <-prCh:
		ticket = createTicket(proof, w.minerAddr)
	}

	// TODO: Test the interplay of isWinningTicket() and createPoST()

	weHaveAWinner, err := consensus.IsWinningTicket(ctx, w.blockstore, w.powerTable, st, ticket, w.minerAddr)

	if err != nil {
		log.Errorf("Worker.Mine couldn't compute ticket: %s", err.Error())
		outCh <- Output{Err: err}
		return false
	}

	if weHaveAWinner {
		next, err := w.Generate(ctx, base, ticket, uint64(nullBlkCount))
		if err == nil {
			log.SetTag(ctx, "block", next)
		}
		log.Debugf("Worker.Mine generates new winning block! %s", next.Cid().String())
		outCh <- NewOutput(next, err)
		return true
	}

	return false
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

func createTicket(proof []byte, minerAddr address.Address) []byte {
	// TODO: the ticket is supposed to be a signature, per the spec.
	// For now to ensure that the ticket is unique to each miner mix in
	// the miner address.
	// https://github.com/filecoin-project/go-filecoin/issues/1054
	buf := append(proof, minerAddr.Bytes()...)
	h := sha256.Sum256(buf)
	return h[:]
}

// fakeCreatePoST is the default implementation of DoSomeWorkFunc.
// It simply sleeps for the blockTime.
func (w *DefaultWorker) fakeCreatePoST() {
	time.Sleep(w.blockTime)
}
