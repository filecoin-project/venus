package vm

import (
	blockstore "github.com/ipfs/go-ipfs-blockstore"

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

// FunctionSignature describes the signature of a single function.
//
// Dragons: this signature should not be neede on the outside
type FunctionSignature = dispatch.FunctionSignature
