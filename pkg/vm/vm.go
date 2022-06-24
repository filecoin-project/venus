package vm

import (
	"context"

	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/register"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
)

// Re-exports

type VmOption = vmcontext.VmOption //nolint

type Ret = vmcontext.Ret

// Interpreter is the LegacyVM.
type Interpreter = vmcontext.VMInterpreter

type SyscallsImpl = vmcontext.SyscallsImpl
type SyscallsStateView = vmcontext.SyscallsStateView

type ExecCallBack = vmcontext.ExecCallBack
type VmMessage = vmcontext.VmMessage //nolint
type FakeSyscalls = vmcontext.FakeSyscalls
type ChainRandomness = vmcontext.HeadChainRandomness

type Interface = vmcontext.Interface // nolint

// NewLegacyVM creates a new LegacyVM interpreter.
func NewLegacyVM(ctx context.Context, option VmOption) (Interpreter, error) {
	if option.ActorCodeLoader == nil {
		option.ActorCodeLoader = GetDefaultActors()
	}

	return vmcontext.NewLegacyVM(ctx, option.ActorCodeLoader, option)
}

// GetDefaultActors return a code loader with the built-in actors that come with the system.
var GetDefaultActors = register.GetDefaultActros

// ActorCodeLoader allows yo to load an actor's code based on its id an epoch.
type ActorCodeLoader = dispatch.CodeLoader

// ActorMethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type ActorMethodSignature = dispatch.MethodSignature

type ILookBack = vmcontext.ILookBack
type LookbackStateGetter = vmcontext.LookbackStateGetter

//type LookbackStateGetterForTipset = vmcontext.LookbackStateGetterForTipset
