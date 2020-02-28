package consensus

import (
	"context"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/crypto"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
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
}

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	actors vm.ActorCodeLoader
	rnd    ChainRandomness
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor(rnd ChainRandomness) *DefaultProcessor {
	return &DefaultProcessor{
		actors: vm.DefaultActors,
		rnd:    rnd,
	}
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader, rnd ChainRandomness) *DefaultProcessor {
	return &DefaultProcessor{
		actors: actors,
		rnd:    rnd,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all blocks in a TipSet.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet, msgs []vm.BlockMessagesInfo) (results []vm.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	epoch, err := ts.Height()
	if err != nil {
		return nil, err
	}

	parent, err := ts.Parents()
	if err != nil {
		return nil, err
	}

	rnd := headRandomness{
		chain: p.rnd,
		head:  parent,
	}
	v := vm.NewVM(st, &vms)

	return v.ApplyTipSetMessages(msgs, epoch, &rnd)
}

// A chain randomness source with a fixed head tipset key.
type headRandomness struct {
	chain ChainRandomness
	head  block.TipSetKey
}

func (h *headRandomness) Randomness(ctx context.Context, tag crypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return h.chain.SampleChainRandomness(ctx, h.head, tag, epoch, entropy)
}
