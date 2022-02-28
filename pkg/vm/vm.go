package vm

import (
	"context"
	"os"

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
type ChainRandomness = vmcontext.HeadChainRandomness

type VMI = vmcontext.VMI // nolint

// NewVenusVM creates a new VM interpreter.
func NewVenusVM(ctx context.Context, option VmOption) (Interpreter, error) {
	if option.ActorCodeLoader == nil {
		option.ActorCodeLoader = &DefaultActors
	}

	return vmcontext.NewVM(ctx, option.ActorCodeLoader, option)
}

func NewVM(ctx context.Context, option VmOption) (VMI, error) {
	if os.Getenv("VENUS_USE_FVM_DOESNT_WORK_YET") == "1" {
		fvmLog.Info("use fvm")
		return NewFVM(ctx, &option)
	}

	return NewVenusVM(ctx, option)
}

// DefaultActors is a code loader with the built-in actors that come with the system.
var DefaultActors = register.DefaultActors

// ActorCodeLoader allows yo to load an actor's code based on its id an epoch.
type ActorCodeLoader = dispatch.CodeLoader

// ActorMethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type ActorMethodSignature = dispatch.MethodSignature

type ILookBack = vmcontext.ILookBack
type LookbackStateGetter = vmcontext.LookbackStateGetter

//type LookbackStateGetterForTipset = vmcontext.LookbackStateGetterForTipset
