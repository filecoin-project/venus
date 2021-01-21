package consensus

import (
	"context"

	"github.com/ipfs/go-cid"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/types"

	//"github.com/filecoin-project/venus/internal/pkg/proofs"
	"github.com/filecoin-project/venus/pkg/vm"
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
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor(syscalls vm.SyscallsImpl) *DefaultProcessor {
	return NewConfiguredProcessor(vm.DefaultActors, syscalls)
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader, syscalls vm.SyscallsImpl) *DefaultProcessor {
	return &DefaultProcessor{
		actors:   actors,
		syscalls: syscalls,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all blocks in a TipSet.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context,
	parent, ts *block.TipSet,
	msgs []block.BlockMessagesInfo,
	vmOption vm.VmOption,
) (cid.Cid, []types.MessageReceipt, error) {
	_, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))

	epoch, err := ts.Height()
	if err != nil {
		return cid.Undef, nil, err
	}

	var parentEpoch abi.ChainEpoch
	if parent.Defined() {
		parentEpoch, err = parent.Height()
		if err != nil {
			return cid.Undef, nil, err
		}
	}

	v, err := vm.NewVM(vmOption)
	if err != nil {
		return cid.Undef, nil, err
	}

	return v.ApplyTipSetMessages(msgs, ts, parentEpoch, epoch, nil)
}

// ProcessTipSet computes the state transition specified by the messages.
func (p *DefaultProcessor) ProcessMessage(ctx context.Context, msg types.ChainMsg, vmOption vm.VmOption) (ret *vm.Ret, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessMessage")
	span.AddAttributes(trace.StringAttribute("process message", msg.VMMessage().String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	v, err := vm.NewVM(vmOption)
	if err != nil {
		return nil, err
	}

	return v.ApplyMessage(msg)
}

func (p *DefaultProcessor) ProcessImplicitMessage(ctx context.Context, msg *types.UnsignedMessage, vmOption vm.VmOption) (ret *vm.Ret, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessImplicitMessage")
	span.AddAttributes(trace.StringAttribute("message", msg.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	v, err := vm.NewVM(vmOption)
	if err != nil {
		return nil, err
	}
	return v.ApplyImplicitMessage(msg)
}
