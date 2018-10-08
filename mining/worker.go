package mining

// The Worker Mines on Input received from a Scheduler.  The Worker is
// responsible for generating the necessary proofs, checking for success,
// generating new blocks, and forwarding them out to the wider node.

import (
	"context"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	sha256 "gx/ipfs/QmXTpwq2AkzQsPjKqFQDNY2bMdsAT53hUBETeyj8QRHTZU/sha256-simd"
	"gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"
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
	Mine(runCtx context.Context, base consensus.TipSet, nullBlkCount int, outCh chan<- Output)
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
func (w *DefaultWorker) Mine(ctx context.Context, base consensus.TipSet, nullBlkCount int, outCh chan<- Output) {
	log.Info("Worker.Mine")
	ctx = log.Start(ctx, "Worker.Mine")
	defer log.Finish(ctx)
	if len(base) == 0 {
		log.Warning("Worker.Mine returning because it can't mine on an empty tipset")
		outCh <- Output{Err: errors.New("bad input tipset with no blocks sent to Mine()")}
		return
	}

	st, err := w.getStateTree(ctx, base)
	if err != nil {
		log.Errorf("Worker.Mine couldn't get state tree for tipset: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}
	totalPower, err := w.powerTable.Total(ctx, st, w.blockstore)
	if err != nil {
		log.Errorf("Worker.Mine couldn't get total power from power table: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}
	myPower, err := w.powerTable.Miner(ctx, st, w.blockstore, w.minerAddr)
	if err != nil {
		log.Errorf("Worker.Mine returning because couldn't get miner power from power table: %s", err.Error())
		outCh <- Output{Err: err}
		return
	}

	log.Debugf("Worker.Mine myPower: %d - totalPower: %d", myPower, totalPower)
	if myPower == 0 {
		// we don't have any power, it doesn't make sense to mine quite yet
		return
	}

	log.Debugf("Mining on tipset: %s, with %d null blocks.", base.String(), nullBlkCount)
	if ctx.Err() != nil {
		log.Warningf("Worker.Mine returning with ctx error %s", ctx.Err().Error())
		return
	}

	challenge, err := createChallenge(base, uint64(nullBlkCount))
	if err != nil {
		outCh <- Output{Err: err}
		return
	}
	prCh := createProof(challenge, w.createPoST)
	var ticket []byte
	select {
	case <-ctx.Done():
		log.Infof("Mining run on base %s with %d null blocks canceled.", base.String(), nullBlkCount)
		return
	case proof := <-prCh:
		ticket = createTicket(proof, w.minerAddr)
	}

	// TODO: Test the interplay of isWinningTicket() and createPoST()
	if isWinningTicket(ticket, int64(myPower), int64(totalPower)) {
		next, err := w.Generate(ctx, base, ticket, uint64(nullBlkCount))
		if err == nil {
			log.SetTag(ctx, "block", next)
		}
		log.Infof("Worker.Mine generates new winning block! %s", next.Cid().String())
		outCh <- NewOutput(next, err)
		return
	}
}

// TODO -- in general this won't work with only the base tipset, we'll potentially
// need some chain manager utils, similar to the State function, to sample
// further back in the chain.
func createChallenge(parents consensus.TipSet, nullBlkCount uint64) ([]byte, error) {
	smallest, err := parents.MinTicket()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4)
	n := binary.PutUvarint(buf, nullBlkCount)
	buf = append(smallest, buf[:n]...)

	h := sha256.Sum256(buf)
	return h[:], nil
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

var isWinningTicket = func(ticket []byte, myPower, totalPower int64) bool {
	// See https://github.com/filecoin-project/aq/issues/70 for an explanation of the math here.
	lhs := &big.Int{}
	lhs.SetBytes(ticket)
	lhs.Mul(lhs, big.NewInt(totalPower))

	rhs := &big.Int{}
	rhs.Mul(big.NewInt(myPower), ticketDomain)
	return lhs.Cmp(rhs) < 0
}

// fakeCreatePoST is the default implementation of DoSomeWorkFunc.
// It simply sleeps for the blockTime.
func (w *DefaultWorker) fakeCreatePoST() {
	time.Sleep(w.blockTime)
}
