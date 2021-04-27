package consensus

import "C"
import (
	"context"
	"time"

	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper"
	"github.com/filecoin-project/venus/pkg/vmsupport"
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

	_ "github.com/filecoin-project/venus/pkg/crypto/sigs/bls"  // enable bls signatures
	_ "github.com/filecoin-project/venus/pkg/crypto/sigs/secp" // enable secp signatures
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
	ProcessMessage(context.Context, types.ChainMsg, vm.VmOption) (*vm.Ret, error)
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
	proofVerifier ffiwrapper.Verifier,
	blockValidator *BlockValidator,
) *Expected {
	faultChecker := consensusfault.NewFaultChecker(chainState, fork)
	syscalls := vmsupport.NewSyscalls(faultChecker, proofVerifier)
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
	}
	return c
}

// RunStateTransition applies the messages in a tipset to a state, and persists that new state.
// It errors if the tipset was not mined according to the EC rules, or if any of the messages
// in the tipset results in an error.
func (c *Expected) RunStateTransition(ctx context.Context,
	ts *types.TipSet,
	parentStateRoot cid.Cid,
) (cid.Cid, cid.Cid, error) {
	ctx, span := trace.StartSpan(ctx, "Expected.RunStateTransition")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))

	blockMessageInfo, err := c.messageStore.LoadTipSetMessage(ctx, ts)
	if err != nil {
		return cid.Undef, cid.Undef, nil
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
		pts, err = c.chainState.GetTipSet(parent)
		if err != nil {
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
		PRoot:             parentStateRoot,
		SysCallsImpl:      c.syscallsImpl,
	}
	root, receipts, err := c.processor.ProcessTipSet(ctx, pts, ts, blockMessageInfo, vmOption)
	if err != nil {
		return cid.Undef, cid.Undef, errors.Wrap(err, "error validating tipset")
	}

	receiptCid, err := c.messageStore.StoreReceipts(ctx, receipts)
	if err != nil {
		return cid.Undef, cid.Undef, xerrors.Errorf("failed to save receipt: %v", err)
	}

	return root, receiptCid, nil
}
