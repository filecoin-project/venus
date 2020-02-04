package dispatch

import (
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
)

// Method is a callable pointer to an executable method in an actor implementation.
type Method interface {
	Call(in []reflect.Value) []reflect.Value
	Type() reflect.Type
}

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Method(id types.MethodID) (Method, *FunctionSignature, bool)
}

// Exports describe the public methods of an actor.
type Exports map[types.MethodID]*FunctionSignature

// FunctionSignature describes the signature of a single function.
type FunctionSignature struct {
	// Params is a list of the types of the parameters the function expects.
	Params []abi.Type
	// Return is the type of the return value of the function.
	Return []abi.Type
}
