package consensus

import "C"
import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/fork"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
)

var (
	ErrExpensiveFork = errors.New("refusing explicit call due to state fork at epoch")
	// ErrStateRootMismatch is returned when the computed state root doesn't match the expected result.
	ErrStateRootMismatch = errors.New("blocks state root does not match computed result")
	// ErrUnorderedTipSets is returned when weight and minticket are the same between two tipsets.
	ErrUnorderedTipSets = errors.New("trying to order two identical tipsets")
	// ErrReceiptRootMismatch is returned when the block's receipt root doesn't match the receipt root computed for the parent tipset.
	ErrReceiptRootMismatch = errors.New("blocks receipt root does not match parent tip set")
)

var logExpect = logging.Logger("consensus")

const AllowableClockDriftSecs = uint64(1)

// A Processor processes all the messages in a block or tip set.
type Processor interface {
	// ProcessTipSet processes all messages in a tip set.
	ProcessTipSet(context.Context, *types.TipSet, *types.TipSet, []types.BlockMessagesInfo, vm.VmOption) (cid.Cid, []types.MessageReceipt, error)
	ProcessImplicitMessage(context.Context, *types.UnsignedMessage, vm.VmOption) (*vm.Ret, error)
}

// TicketValidator validates that an input ticket is valid.
type TicketValidator interface {
	IsValidTicket(ctx context.Context, base types.TipSetKey, entry *types.BeaconEntry, newPeriod bool, epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket types.Ticket) error
}

// Todo Delete view just use state.Viewer
// AsDefaultStateViewer adapts a state viewer to a power state viewer.
func AsDefaultStateViewer(v *appstate.Viewer) DefaultStateViewer {
	return DefaultStateViewer{v}
}

// DefaultStateViewer a state viewer to the power state view interface.
type DefaultStateViewer struct {
	*appstate.Viewer
}

// PowerStateView returns a power state view for a state root.
func (v *DefaultStateViewer) PowerStateView(root cid.Cid) appstate.PowerStateView {
	return v.Viewer.StateView(root)
}

// FaultStateView returns a fault state view for a state root.
func (v *DefaultStateViewer) FaultStateView(root cid.Cid) appstate.FaultStateView {
	return v.Viewer.StateView(root)
}

// StateViewer provides views into the Chain state.
type StateViewer interface {
	PowerStateView(root cid.Cid) appstate.PowerStateView
	FaultStateView(root cid.Cid) appstate.FaultStateView
}

type chainReader interface {
	GetTipSet(types.TipSetKey) (*types.TipSet, error)
	GetHead() *types.TipSet
	GetTipSetStateRoot(*types.TipSet) (cid.Cid, error)
	GetTipSetReceiptsRoot(*types.TipSet) (cid.Cid, error)
	GetGenesisBlock(context.Context) (*types.BlockHeader, error)
	GetLatestBeaconEntry(*types.TipSet) (*types.BeaconEntry, error)
	GetTipSetByHeight(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error)
	GetCirculatingSupplyDetailed(context.Context, abi.ChainEpoch, tree.Tree) (chain.CirculatingSupply, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
}

type stateComputResult struct {
	stateRoot, receipt cid.Cid
}

// Expected implements expected consensus.
type Expected struct {
	// cstore is used for loading state trees during message running.
	cstore cbor.IpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstore.Blockstore

	// message store for message read/write
	messageStore *chain.MessageStore

	// chainState is a reference to the current Chain state
	chainState chainReader

	// processor is what we use to process messages and pay rewards
	processor Processor

	// calculate chain randomness ticket/beacon
	rnd ChainRandomness

	// fork for vm process and block validator
	fork fork.IFork

	// gas price for vm
	gasPirceSchedule *gas.PricesSchedule

	// circulate supply calculator for vm
	circulatingSupplyCalculator *chain.CirculatingSupplyCalculator

	// systemcall for vm
	syscallsImpl vm.SyscallsImpl

	// block validator before process tipset
	blockValidator *BlockValidator

	stCache       map[types.TipSetKey]stateComputResult
	stComputeChs  map[types.TipSetKey]chan struct{}
	stComputeLock sync.Mutex
}

// Ensure Expected satisfies the Protocol interface at compile time.
var _ Protocol = (*Expected)(nil)

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore,
	bs blockstore.Blockstore,
	bt time.Duration,
	chainState chainReader,
	rnd ChainRandomness,
	messageStore *chain.MessageStore,
	fork fork.IFork,
	config *config.NetworkParamsConfig,
	gasPirceSchedule *gas.PricesSchedule,
	blockValidator *BlockValidator,
	syscalls vm.SyscallsImpl,
) *Expected {
	processor := NewDefaultProcessor(syscalls)
	c := &Expected{
		processor:                   processor,
		syscallsImpl:                syscalls,
		cstore:                      cs,
		bstore:                      bs,
		chainState:                  chainState,
		messageStore:                messageStore,
		rnd:                         rnd,
		fork:                        fork,
		gasPirceSchedule:            gasPirceSchedule,
		blockValidator:              blockValidator,
		circulatingSupplyCalculator: chain.NewCirculatingSupplyCalculator(bs, chainState, config.ForkUpgradeParam),

		stCache:      make(map[types.TipSetKey]stateComputResult),
		stComputeChs: make(map[types.TipSetKey]chan struct{}),
	}
	return c
}

func (c *Expected) RunStateTransition(ctx context.Context, ts *types.TipSet) (cid.Cid, cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "Exected.RunStateTransition")
	defer span.End()

	var state stateComputResult
	var err error
	key := ts.Key()
	c.stComputeLock.Lock()

	cmptCh, exist := c.stComputeChs[key]

	if exist {
		c.stComputeLock.Unlock()
		select {
		case <-cmptCh:
			c.stComputeLock.Lock()
		case <-ctx.Done():
			return cid.Undef, cid.Undef, ctx.Err()
		}
	}

	if state, exist = c.stCache[key]; exist {
		c.stComputeLock.Unlock()
		return state.stateRoot, state.receipt, nil
	}

	cmptCh = make(chan struct{})
	c.stComputeChs[key] = cmptCh
	c.stComputeLock.Unlock()

	defer func() {
		c.stComputeLock.Lock()
		delete(c.stComputeChs, key)
		if !state.stateRoot.Equals(cid.Undef) {
			c.stCache[key] = state
		}
		c.stComputeLock.Unlock()
		close(cmptCh)
	}()

	if ts.Height() == 0 {
		return ts.Blocks()[0].ParentStateRoot, ts.Blocks()[0].ParentMessageReceipts, nil
	}

	if state.stateRoot, state.receipt, err = c.runStateTransition(ctx, ts); err != nil {
		return cid.Undef, cid.Undef, err
	}

	return state.stateRoot, state.receipt, nil

}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) runStateTransition(ctx context.Context, ts *types.TipSet) (cid.Cid, cid.Cid, error) {
	fmt.Printf("_sc| runstatetransition begin(%d)\n_sc|", ts.Height())
	logbuf := &strings.Builder{}
	begin := time.Now()

	defer func() {
		_, _ = fmt.Fprintf(logbuf,
			`_sc| total cost time = %.4f(seconds)
_sc|-------------------------------------
_sc|`, time.Since(begin).Seconds())
		fmt.Printf(logbuf.String())
	}()

	_, _ = fmt.Fprintf(logbuf, "_sc|______RunStateTransaction(%d) details____________________\n"+
		"_sc| tipset key:%s\n", ts.Height(), ts.Key().String())

	ctx, span := trace.StartSpan(ctx, "Expected.innerRunStateTransition")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("blocks", ts.String()))
	span.AddAttributes(trace.Int64Attribute("height", int64(ts.Height())))

	beginLoadTsMessage := time.Now()
	blockMessageInfo, err := c.messageStore.LoadTipSetMessage(ctx, ts)
	if err != nil {
		_, _ = fmt.Fprintf(logbuf, "_sc| load tipset message failed::%s\n", err.Error())
		return cid.Undef, cid.Undef, err
	}
	_, _ = fmt.Fprintf(logbuf, "_sc| load tipset message cost time:%.4f(s)\n",
		time.Since(beginLoadTsMessage).Seconds())
	// process tipset
	var pts *types.TipSet

	if ts.Height() == 0 {
		// NB: This is here because the process that executes blocks requires that the
		// block miner reference a valid miner in the state tree. Unless we create some
		// magical genesis miner, this won't work properly, so we short circuit here
		// This avoids the question of 'who gets paid the genesis block reward'
		return ts.Blocks()[0].ParentStateRoot, ts.Blocks()[0].ParentMessageReceipts, nil
	} else if ts.Height() > 0 {
		parent := ts.Parents()
		if pts, err = c.chainState.GetTipSet(parent); err != nil {
			_, _ = fmt.Fprintf(logbuf, "_sc| get parent tipset failed:%s\n", err.Error())
			return cid.Undef, cid.Undef, err
		}
	} else {
		return cid.Undef, cid.Undef, nil
	}

	rnd := HeadRandomness{
		Chain: c.rnd,
		Head:  ts.Key(),
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			dertail, err := c.chainState.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Fork:              c.fork,
		Epoch:             ts.At(0).Height,
		GasPriceSchedule:  c.gasPirceSchedule,
		Bsstore:           c.bstore,
		PRoot:             ts.At(0).ParentStateRoot,
		SysCallsImpl:      c.syscallsImpl,
	}

	beginProcessTipset := time.Now()
	root, receipts, err := c.processor.ProcessTipSet(ctx, pts, ts, blockMessageInfo, vmOption)
	if err != nil {
		_, _ = fmt.Fprintf(logbuf, "_sc| processTipset failed:%s\n", err.Error())
		return cid.Undef, cid.Undef, errors.Wrap(err, "error validating tipset")
	}
	_, _ = fmt.Fprintf(logbuf, "_sc| processTipset cost time:%.4f(s)\n",
		time.Since(beginProcessTipset).Seconds())

	beginStoreReceipts := time.Now()
	receiptCid, err := c.messageStore.StoreReceipts(ctx, receipts)
	if err != nil {
		_, _ = fmt.Fprintf(logbuf, "_sc| store receipts failed:%s\n", err.Error())
		return cid.Undef, cid.Undef, xerrors.Errorf("failed to save receipt: %v", err)
	}
	_, _ = fmt.Fprintf(logbuf, "_sc| storeReceipts cost time:%.4f(s)\n",
		time.Since(beginStoreReceipts).Seconds())

	return root, receiptCid, nil
}
