package dispatch

import (
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/ipfs/go-cid"
)

// Actor is the interface all actors have to implement.
type Actor interface {
	// Exports has a list of method available on the actor.
	Exports() []interface{}
}

// Dispatcher allows for dynamic method dispatching on an actor.
type Dispatcher interface {
	// Dispatch will call the given method on the actor and pass the arguments.
	//
	// - The `ctx` argument will be coerced to the type the method expects in its first argument.
	// - If arg1 is `[]byte`, it will attempt to decode the value based on second argument in the target method.
	Dispatch(method types.MethodID, ctx interface{}, arg1 interface{}) (interface{}, error)
	// Signature is a helper function taht returns the signature for a given method.
	//
	// Note: This is intended to be used by tests and tools.
	Signature(method types.MethodID) (MethodSignature, error)
}

type actorDispatcher struct {
	code  cid.Cid
	actor Actor
}

type method interface {
	Call(in []reflect.Value) []reflect.Value
	Type() reflect.Type
}

var _ Dispatcher = (*actorDispatcher)(nil)

// Dispatch implements `Dispatcher`.
func (d *actorDispatcher) Dispatch(methodID types.MethodID, ctx interface{}, arg1 interface{}) (interface{}, error) {
	// get method signature
	m, err := d.signature(methodID)
	if err != nil {
		return nil, err
	}

	// build args to pass to the method
	args := []reflect.Value{
		// the ctx will be automaticall coerced
		reflect.ValueOf(ctx),
	}

	if raw, ok := arg1.([]byte); ok {
		obj, err := m.ArgInterface(raw)
		if err != nil {
			return nil, err
		}

		// push decoded arg to args list
		args = append(args, reflect.ValueOf(obj))
	} else {
		// the argument was not in raw bytes, let it be coerced
		args = append(args, reflect.ValueOf(arg1))
	}

	// invoke the method
	out := m.method.Call(args)

	// Note: we only support single objects being returned
	if len(out) > 1 {
		return nil, fmt.Errorf("actor method returned more than one object. method: %d, code: %s", methodID, d.code)
	}

	// method returns unit
	if len(out) == 0 {
		return nil, nil
	}

	// forward return
	return out[1].Interface(), nil
}

func (d *actorDispatcher) signature(methodID types.MethodID) (*methodSignature, error) {
	exports := d.actor.Exports()

	// get method entry
	methodIdx := (uint64)(methodID)
	if len(exports) < (int)(methodIdx) {
		return nil, fmt.Errorf("Method undefined. method: %d, code: %s", methodID, d.code)
	}
	entry := exports[methodIdx]
	if entry == nil {
		return nil, fmt.Errorf("Method undefined. method: %d, code: %s", methodID, d.code)
	}

	// for the moment, we only support dispatching if the entry in the export table is a method pointer.
	m, ok := entry.(method)
	if !ok {
		// Note: the entry defined in the actor code is not a method pointer, check the `spec-actors` repo code.
		return nil, fmt.Errorf("Unsupported method definition. method: %d, code: %s", methodID, d.code)
	}

	return &methodSignature{method: m}, nil
}

// Signature implements `Dispatcher`.
func (d *actorDispatcher) Signature(methodID types.MethodID) (MethodSignature, error) {
	return d.Signature(methodID)
}
