package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
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

// DefaultProcessor handles all block processing.
type DefaultProcessor struct {
	actors vm.ActorCodeLoader
}

var _ Processor = (*DefaultProcessor)(nil)

// NewDefaultProcessor creates a default processor from the given state tree and vms.
func NewDefaultProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		actors: vm.DefaultActors,
	}
}

// NewConfiguredProcessor creates a default processor with custom validation and rewards.
func NewConfiguredProcessor(actors vm.ActorCodeLoader) *DefaultProcessor {
	return &DefaultProcessor{
		actors: actors,
	}
}

// ProcessTipSet computes the state transition specified by the messages in all
// blocks in a TipSet.  It is similar to ProcessBlock with a few key differences.
// Most importantly ProcessTipSet relies on the precondition that each input block
// is valid with respect to the base state st, that is, ProcessBlock is free of
// errors when applied to each block individually over the given state.
// ProcessTipSet only returns errors in the case of faults.  Other errors
// coming from calls to ApplyMessage can be traced to different blocks in the
// TipSet containing conflicting messages and are returned in the result slice.
// Blocks are applied in the sorted order of their tickets.
func (p *DefaultProcessor) ProcessTipSet(ctx context.Context, st state.Tree, vms vm.Storage, ts block.TipSet, msgs []vm.BlockMessagesInfo) (results []vm.MessageReceipt, err error) {
	ctx, span := trace.StartSpan(ctx, "DefaultProcessor.ProcessTipSet")
	span.AddAttributes(trace.StringAttribute("tipset", ts.String()))
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	h, err := ts.Height()
	if err != nil {
		return nil, fmt.Errorf("processing empty tipset")
	}
	epoch := types.NewBlockHeight(h)

	vm := vm.NewVM(st, &vms)

	return vm.ApplyTipSetMessages(msgs, *epoch)
}

// ResolveAddress looks up associated id address. If the given address is already and id address, it is returned unchanged.
func ResolveAddress(ctx context.Context, addr address.Address, st *state.CachedTree, vms vm.Storage) (address.Address, bool, error) {
	// Dragons: remove completly, or just FW to the vm. we can expose a method on the VM that lets you do this.
	return address.Undef, true, nil
}
