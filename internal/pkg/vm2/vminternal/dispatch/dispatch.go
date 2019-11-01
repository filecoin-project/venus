package dispatch

import (
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/external"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/vminternal/runtime"
)

// Method is a callable pointer to an executable method in an actor implementation.
type Method interface {
	Call(in []reflect.Value) []reflect.Value
	Type() reflect.Type
}

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Method(id types.MethodID) (Method, *external.FunctionSignature, bool)
	InitializeState(storage runtime.Storage, initializerData interface{}) error
}

// Exports describe the public methods of an actor.
type Exports map[types.MethodID]*external.FunctionSignature

// ExportedFunc is the signature an exported method of an actor is expected to have.
type ExportedFunc func(ctx runtime.Runtime) ([]byte, uint8, error)
