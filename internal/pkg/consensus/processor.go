package consensus

import (
	"context"

	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/internal/pkg/types"
	//"github.com/filecoin-project/venus/internal/pkg/proofs"
	"github.com/filecoin-project/venus/internal/pkg/vm"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
)

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *types.MessageReceipt
	ExecutionError error
}

// ApplyMessageResult is the result of applying a single message.
type ApplyMessageResult struct {
	ApplicationResult        // Application-level result, if error is nil.
	Failure            error // Failure to apply the message
	FailureIsPermanent bool  // Whether failure is permanent, has no chance of succeeding later.
}

type ChainRandomness interface {
	SampleChainRandomness(ctx context.Context, head block.TipSetKey, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	actors   vm.ActorCodeLoader
	syscalls vm.SyscallsImpl
	rnd      ChainRandomness
	// fork     fork.IFork
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor(syscalls vm.SyscallsImpl, rnd ChainRandomness) *DefaultProcessor {
	return NewConfiguredProcessor(vm.DefaultActors, syscalls, rnd)
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader, syscalls vm.SyscallsImpl, rnd ChainRandomness) *DefaultProcessor {
	return &DefaultProcessor{
		actors:   actors,
		syscalls: syscalls,
		rnd:      rnd,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all blocks in a TipSet.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms *vm.Storage, parent, ts *block.TipSet, msgs []vm.BlockMessagesInfo, vmOption vm.VmOption) (results []types.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	epoch, err := ts.Height()
	if err != nil {
		return nil, err
	}

	var parentEpoch abi.ChainEpoch
	if parent.Defined() {
		parentEpoch, err = parent.Height()
		if err != nil {
			return nil, err
		}
	}

	v := vm.NewVM(st, vms, p.syscalls, vmOption)

	return v.ApplyTipSetMessages(msgs, ts, parentEpoch, epoch, nil)
}

// ProcessTipSet computes the state transition specified by the messages.
func (p *DefaultProcessor) ProcessUnsignedMessage(ctx context.Context, msg *types.UnsignedMessage, st state.Tree, vms *vm.Storage, vmOption vm.VmOption) (ret *vm.Ret, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessUnsignedMessage")
	span.AddAttributes(trace.StringAttribute("unsignedmessage", msg.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	v := vm.NewVM(st, vms, p.syscalls, vmOption)
	ret = v.ApplyMessage(msg)
	return ret, nil
}
