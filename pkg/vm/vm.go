package vm

import (
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/register"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
)

// Re-exports

type VmOption = vmcontext.VmOption //nolint

type Ret = vmcontext.Ret

// Interpreter is the VM.
type Interpreter = vmcontext.VMInterpreter

type SyscallsImpl = vmcontext.SyscallsImpl
type SyscallsStateView = vmcontext.SyscallsStateView

type ExecCallBack = vmcontext.ExecCallBack
type VmMessage = vmcontext.VmMessage //nolint
type FakeSyscalls = vmcontext.FakeSyscalls

// NewVM creates a new VM interpreter.
func NewVM(option VmOption) (Interpreter, error) {
	if option.ActorCodeLoader == nil {
		option.ActorCodeLoader = &DefaultActors
	}

	return vmcontext.NewVM(option.ActorCodeLoader, option)
}

// DefaultActors is a code loader with the built-in actors that come with the system.
var DefaultActors = register.DefaultActors

// ActorCodeLoader allows yo to load an actor's code based on its id an epoch.
type ActorCodeLoader = dispatch.CodeLoader

// ActorMethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type ActorMethodSignature = dispatch.MethodSignature
