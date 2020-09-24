package consensus

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
)

// ApplicationResult contains the result of successfully applying one message.
// ExecutionError might be set and the message can still be applied successfully.
// See ApplyMessage() for details.
type ApplicationResult struct {
	Receipt        *vm.MessageReceipt
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
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet, msgs []vm.BlockMessagesInfo) (results []vm.MessageReceipt, err error) {
	//ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	//span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	//defer tracing.AddErrorEndSpan(ctx, span, &err)
	//
	//epoch, err := ts.Height()
	//if err != nil {
	//	return nil, err
	//}
	//
	//parent, err := ts.Parents()
	//if err != nil {
	//	return nil, err
	//}
	//
	//// Note: since the parent tipset key is now passed explicitly to ApplyTipSetMessages we can refactor to skip
	//// currying it in to the randomness call here.
	//rnd := headRandomness{
	//	chain: p.rnd,
	//	head:  parent,
	//}
	//v := vm.NewVM(st, &vms, p.syscalls, abi.NewTokenAmount(0))

	// return v.ApplyTipSetMessages(msgs, parent, epoch, &rnd)
	return nil, nil
}

// A chain randomness source with a fixed head tipset key.
type headRandomness struct {
	chain ChainRandomness
	head  block.TipSetKey
}

func (h *headRandomness) Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.SampleChainRandomness(ctx, h.head, tag, epoch, entropy)
}
