package dispatch

import (
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm2/external"
)

// Method is a callable pointer to an executable method in an actor implementation.
type Method interface {
	Call(in []reflect.Value) []reflect.Value
	Type() reflect.Type
}

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Method(id types.MethodID) (Method, *external.FunctionSignature, bool)
	InitializeState(storage vm2.Storage, initializerData interface{}) error
}

// Exports describe the public methods of an actor.
type Exports map[types.MethodID]*external.FunctionSignature

// ExportedFunc is the signature an exported method of an actor is expected to have.
type ExportedFunc func(ctx vm2.Runtime) ([]byte, uint8, error)
