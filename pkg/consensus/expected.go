package consensus

import (
	"context"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/fork"
	appstate "github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/venus-shared/types"
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
	// ApplyBlocks processes all messages in a tip set.
	ApplyBlocks(ctx context.Context, blocks []types.BlockMessagesInfo, ts *types.TipSet, pstate cid.Cid, parentEpoch, epoch abi.ChainEpoch, vmOpts vm.VmOption, cb vm.ExecCallBack) (cid.Cid, []types.MessageReceipt, error)
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
	GetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)
	GetHead() *types.TipSet
	StateView(ctx context.Context, ts *types.TipSet) (*appstate.View, error)
	GetTipSetStateRoot(context.Context, *types.TipSet) (cid.Cid, error)
	GetTipSetReceiptsRoot(context.Context, *types.TipSet) (cid.Cid, error)
	GetGenesisBlock(context.Context) (*types.BlockHeader, error)
	GetLatestBeaconEntry(context.Context, *types.TipSet) (*types.BeaconEntry, error)
	GetTipSetByHeight(context.Context, *types.TipSet, abi.ChainEpoch, bool) (*types.TipSet, error)
	GetCirculatingSupplyDetailed(context.Context, abi.ChainEpoch, tree.Tree) (types.CirculatingSupply, error)
	GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error)
	GetTipsetMetadata(context.Context, *types.TipSet) (*chain.TipSetMetadata, error)
	PutTipSetMetadata(context.Context, *chain.TipSetMetadata) error
}

var _ chainReader = (*chain.Store)(nil)

// Expected implements expected consensus.
type Expected struct {
	// cstore is used for loading state trees during message running.
	cstore cbor.IpldStore

	// bstore contains data referenced by actors within the state
	// during message running.  Additionally bstore is used for
	// accessing the power table.
	bstore blockstoreutil.Blockstore

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

	// systemcall for vm
	syscallsImpl vm.SyscallsImpl

	// block validator before process tipset
	blockValidator *BlockValidator
}

// NewExpected is the constructor for the Expected consenus.Protocol module.
func NewExpected(cs cbor.IpldStore,
	bs blockstoreutil.Blockstore,
	chainState *chain.Store,
	rnd ChainRandomness,
	messageStore *chain.MessageStore,
	fork fork.IFork,
	gasPirceSchedule *gas.PricesSchedule,
	blockValidator *BlockValidator,
	syscalls vm.SyscallsImpl,
	circulatingSupplyCalculator chain.ICirculatingSupplyCalcualtor,
) *Expected {
	processor := NewDefaultProcessor(syscalls, circulatingSupplyCalculator)
	return &Expected{
		processor:        processor,
		syscallsImpl:     syscalls,
		cstore:           cs,
		bstore:           bs,
		chainState:       chainState,
		messageStore:     messageStore,
		rnd:              rnd,
		fork:             fork,
		gasPirceSchedule: gasPirceSchedule,
		blockValidator:   blockValidator,
	}
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context, ts *types.TipSet) (cid.Cid, cid.Cid, error) {
	begin := time.Now()
	defer func() {
		logExpect.Infof("expected.runstatetransition(height:%d, blocks:%d), cost time = %.4f(s)",
			ts.Height(), ts.Len(), time.Since(begin).Seconds())
	}()
	ctx, span := trace.StartSpan(ctx, "Expected.innerRunStateTransition")
	defer span.End()
	span.AddAttributes(trace.StringAttribute("blocks", ts.String()))
	span.AddAttributes(trace.Int64Attribute("height", int64(ts.Height())))
	blockMessageInfo, err := c.messageStore.LoadTipSetMessage(ctx, ts)
	if err != nil {
		return cid.Undef, cid.Undef, err
	}
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
		if pts, err = c.chainState.GetTipSet(ctx, parent); err != nil {
			return cid.Undef, cid.Undef, err
		}
	} else {
		return cid.Undef, cid.Undef, nil
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			dertail, err := c.chainState.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		LookbackStateGetter: vmcontext.LookbackStateGetterForTipset(ctx, c.chainState, c.fork, ts),
		NetworkVersion:      c.fork.GetNetworkVersion(ctx, ts.At(0).Height),
		Rnd:                 NewHeadRandomness(c.rnd, ts.Key()),
		BaseFee:             ts.At(0).ParentBaseFee,
		Fork:                c.fork,
		Epoch:               ts.At(0).Height,
		GasPriceSchedule:    c.gasPirceSchedule,
		Bsstore:             c.bstore,
		PRoot:               ts.At(0).ParentStateRoot,
		SysCallsImpl:        c.syscallsImpl,
	}

	var parentEpoch abi.ChainEpoch
	if pts.Defined() {
		parentEpoch = pts.Height()
	}

	root, receipts, err := c.processor.ApplyBlocks(ctx, blockMessageInfo, ts, ts.ParentState(), parentEpoch, ts.Height(), vmOption, nil)
	if err != nil {
		return cid.Undef, cid.Undef, errors.Wrap(err, "error validating tipset")
	}

	receiptCid, err := c.messageStore.StoreReceipts(ctx, receipts)
	if err != nil {
		return cid.Undef, cid.Undef, fmt.Errorf("failed to save receipt: %v", err)
	}

	return root, receiptCid, nil
}
