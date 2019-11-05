package procneeds

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
)

// GasTracker maintains the state of gas usage throughout the execution of a block and a message
type GasTracker = gastracker.GasTracker

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker() *gastracker.GasTracker {
	return gastracker.NewGasTracker()
}

// NewContextParams is passed to NewVMContext to construct a new context.
type NewContextParams = vmcontext.NewContextParams

// NewVMContext returns an initialized context.
func NewVMContext(params NewContextParams) *vmcontext.VMContext {
	return vmcontext.NewVMContext(params)
}

//
// Free functions
//

// Send executes a message pass inside the VM. If error is set it
// will always satisfy either ShouldRevert() or IsFault().
func Send(ctx context.Context, vmCtx vmcontext.Panda) ([][]byte, uint8, error) {
	return vmcontext.Send(ctx, vmCtx)
}

// Transfer transfers the given value between two actors.
func Transfer(fromActor, toActor *actor.Actor, value types.AttoFIL) error {
	return vmcontext.Transfer(fromActor, toActor, value)
}
