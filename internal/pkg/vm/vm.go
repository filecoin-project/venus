package vm

import (
	"context"

	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Re-exports

// Interpreter is the VM.
type Interpreter = interpreter.VMInterpreter

// Storage is the raw storage for the VM.
type Storage = storage.VMStorage

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo = interpreter.BlockMessagesInfo

// MessageReceipt is what is returned by executing a message on the vm.
type MessageReceipt = message.Receipt

// NewVM creates a new VM interpreter.
func NewVM(st state.Tree, store *storage.VMStorage) Interpreter {
	vm := vmcontext.NewVM(vmcontext.NewProdRandomnessSource(), vmcontext.NewProdActorImplTable(), store, st)
	return &vm
}

// NewStorage creates a new Storage for the VM.
func NewStorage(bs blockstore.Blockstore) Storage {
	return storage.NewStorage(bs)
}

// StorageMap manages Storages.
type StorageMap = storagemap.StorageMap

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return storagemap.NewStorageMap(bs)
}

// LegacyGasTracker maintains the state of gas usage throughout the execution of a block and a message
type LegacyGasTracker = gastracker.LegacyGasTracker

// NewLegacyGasTracker initializes a new empty gas tracker
func NewLegacyGasTracker() *gastracker.LegacyGasTracker {
	return gastracker.NewLegacyGasTracker()
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
func Send(ctx context.Context, vmCtx *vmcontext.VMContext) ([][]byte, uint8, error) {
	return vmcontext.LegacySend(ctx, vmCtx)
}

// Transfer transfers the given value between two actors.
func Transfer(fromActor, toActor *actor.Actor, value types.AttoFIL) error {
	return vmcontext.Transfer(fromActor, toActor, value)
}

// FunctionSignature describes the signature of a single function.
//
// Dragons: this signature should not be neede on the outside
type FunctionSignature = dispatch.FunctionSignature
