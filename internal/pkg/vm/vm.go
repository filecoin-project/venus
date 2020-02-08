package vm

import (
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
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
	vm := vmcontext.NewVM(vmcontext.NewProdRandomnessSource(), dispatcher.DefaultActors, store, st)
	return &vm
}

// NewStorage creates a new Storage for the VM.
func NewStorage(bs blockstore.Blockstore) Storage {
	return storage.NewStorage(bs)
}

// DefaultActors is a code loader with the built-in actors that come with the system.
var DefaultActors = dispatch.DefaultActors

// ActorCodeLoader allows yo to load an actor's code based on its id an epoch.
type ActorCodeLoader = dispatch.CodeLoader

// ActorMethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type ActorMethodSignature = dispatch.MethodSignature
