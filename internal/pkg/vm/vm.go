package vm

import (
	"context"

	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
)

// Re-exports

// StorageMap manages Storages.
type StorageMap = storagemap.StorageMap

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return storagemap.NewStorageMap(bs)
}

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
func Send(ctx context.Context, vmCtx vmcontext.ExtendedRuntime) ([][]byte, uint8, error) {
	return vmcontext.Send(ctx, vmCtx)
}

// Transfer transfers the given value between two actors.
func Transfer(fromActor, toActor *actor.Actor, value types.AttoFIL) error {
	return vmcontext.Transfer(fromActor, toActor, value)
}

// FunctionSignature describes the signature of a single function.
//
// Dragons: this signature should not be neede on the outside
type FunctionSignature = dispatch.FunctionSignature
