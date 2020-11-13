package vm

import (
	"github.com/filecoin-project/venus/internal/pkg/vm/register"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/venus/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/vmcontext"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
	"github.com/filecoin-project/venus/internal/pkg/vm/storage"
)

// Re-exports

type VmOption = vmcontext.VmOption //nolint

type Ret = vmcontext.Ret

// Interpreter is the VM.
type Interpreter = vmcontext.VMInterpreter

// Storage is the raw storage for the VM.
type Storage = storage.VMStorage

type SyscallsImpl = vmcontext.SyscallsImpl
type SyscallsStateView = vmcontext.SyscallsStateView

// BlockMessagesInfo contains messages for one block in a tipset.
type BlockMessagesInfo = vmcontext.BlockMessagesInfo

type ExecCallBack = vmcontext.ExecCallBack
type VmMessage = vmcontext.VmMessage //nolint
type FakeSyscalls = vmcontext.FakeSyscalls

// NewVM creates a new VM interpreter.
func NewVM(st state.Tree, store *storage.VMStorage, syscalls SyscallsImpl, option VmOption) Interpreter {
	if option.ActorCodeLoader == nil {
		option.ActorCodeLoader = &DefaultActors
	}

	vm := vmcontext.NewVM(option.ActorCodeLoader, store, st, syscalls, option)
	return &vm
}

// NewStorage creates a new Storage for the VM.
func NewStorage(bs blockstore.Blockstore) *Storage {
	return storage.NewStorage(bs)
}

// DefaultActors is a code loader with the built-in actors that come with the system.
var DefaultActors = register.DefaultActors

// ActorCodeLoader allows yo to load an actor's code based on its id an epoch.
type ActorCodeLoader = dispatch.CodeLoader

// ActorMethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type ActorMethodSignature = dispatch.MethodSignature
