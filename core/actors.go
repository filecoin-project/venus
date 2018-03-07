package core

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

// BuiltinActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
var BuiltinActors = map[string]ExecutableActor{}

// Exports describe the public methods of an actor.
type Exports map[string]*FunctionSignature

// Has checks if the given method is an exported method.
func (e Exports) Has(method string) bool {
	_, ok := e[method]
	return ok
}

// TODO fritz require actors to define their exit codes and associate
// an error string with them.

// ExecutableActor is the interface all builtin actors have to implement.
type ExecutableActor interface {
	Exports() Exports
}

// ExportedFunc is the signature an exported method of an actor is expected to have.
type ExportedFunc func(ctx *VMContext) ([]byte, uint8, error)

// FunctionSignature describes the signature of a single function.
// TODO: convert signatures into non go types, but rather low level agreed up types
type FunctionSignature struct {
	// Params is a list of the types of the parameters the function expects.
	Params []abi.Type
	// Return is the type of the return value of the function.
	Return []abi.Type
}

func init() {
	// Instance Actors
	// TODO: these should probably not be direct instances, but constructors for the actors
	BuiltinActors[types.AccountActorCodeCid.KeyString()] = &AccountActor{}
	BuiltinActors[types.StorageMarketActorCodeCid.KeyString()] = &StorageMarketActor{}
	BuiltinActors[types.MinerActorCodeCid.KeyString()] = &MinerActor{}
}

// LoadCode fetches the code referenced by the passed in CID.
func LoadCode(codePointer *cid.Cid) (ExecutableActor, error) {
	if codePointer == nil {
		return nil, fmt.Errorf("missing code")
	}
	actor, ok := BuiltinActors[codePointer.KeyString()]
	if !ok {
		return nil, fmt.Errorf("unknown code: %s", codePointer.String())
	}

	return actor, nil
}

// MakeTypedExport finds the correct method on the given actor and returns it.
// The returned function is wrapped such that it takes care of serialization and type checks.
//
// TODO: the work of creating the wrapper should be ideally done at compile time, otherwise at least only once + cached
// TODO: find a better name, naming is hard..
func MakeTypedExport(actor ExecutableActor, method string) ExportedFunc {
	f, ok := reflect.TypeOf(actor).MethodByName(strings.Title(method))
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in actor: %s", method))
	}

	exports := actor.Exports()
	signature, ok := exports[method]
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in exports: %s", method))
	}

	val := f.Func
	t := f.Type
	// number of input args, struct receiver + context + dynamic params
	numIn := 2 + len(signature.Params)

	if t.Kind() != reflect.Func || t.NumIn() != numIn {
		fmt.Println(t.Kind())
		panic(fmt.Sprintf("MakeTypedExport must receive a function with %d parameters for %s", numIn, method))
	}

	exitType := reflect.Uint8
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	if signature.Return != nil {
		if len(signature.Return) != 1 {
			panic("FunctionSignature.Return must be either nil or have exactly one value")
		}

		if t.NumOut() != 3 || !abi.TypeMatches(signature.Return[0], t.Out(0)) ||
			t.Out(1).Kind() != exitType || !t.Out(2).Implements(errorType) {
			panic(fmt.Sprintf("MakeTypedExport must receive a function that returns (%s, uint8, error) for %s", signature.Return[0], method))
		}
	} else {
		if t.NumOut() != 2 || t.Out(0).Kind() != exitType || !t.Out(1).Implements(errorType) {
			panic(fmt.Sprintf("MakeTypedExport must receive a function that returns (uint8, error) for %s", method))
		}
	}

	return func(ctx *VMContext) ([]byte, uint8, error) {
		params, err := abi.DecodeValues(ctx.Message().Params, signature.Params)
		if err != nil {
			return nil, 1, faultErrorWrap(err, "invalid params")
		}

		args := []reflect.Value{
			reflect.ValueOf(actor),
			reflect.ValueOf(ctx),
		}

		for _, param := range params {
			args = append(args, reflect.ValueOf(param.Val))
		}

		out := val.Call(args)

		var retVal []byte
		if signature.Return != nil {
			ret, err := marshalValue(out[0].Interface())
			if err != nil {
				return nil, 1, faultErrorWrap(err, "failed to marshal output value")
			}

			retVal = ret
			out = out[1:]
		}

		exitCode, ok := out[0].Interface().(uint8)
		if !ok {
			panic("invalid return value")
		}

		outErr, ok := out[1].Interface().(error)
		if ok {
			if !(shouldRevert(outErr) || IsFault(outErr)) {
				panic("you are a bad person: error must be either a reverterror or a fault")
			}
		} else {
			// The value of the returned error was nil.
			outErr = nil
		}

		return retVal, exitCode, outErr
	}
}

// marshalValue serializes a given go type into a byte slice.
// The returned format matches the format that is expected to be interoperapble between VM and
// the rest of the system.
func marshalValue(val interface{}) ([]byte, error) {
	switch t := val.(type) {
	case *big.Int:
		if t == nil {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	case []byte:
		return t, nil
	case string:
		return []byte(t), nil
	case types.Address:
		if t == (types.Address{}) {
			return []byte{}, nil
		}
		return t.Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown type: %s", reflect.TypeOf(t))
	}
}

// --
// Below are helper functions that are used to implement actors.

// MarshalStorage encodes the passed in data into bytes.
func MarshalStorage(in interface{}) ([]byte, error) {
	return cbor.DumpObject(in)
}

// UnmarshalStorage decodes the passed in bytes into the given object.
func UnmarshalStorage(raw []byte, to interface{}) error {
	return cbor.DecodeInto(raw, to)
}

// WithStorage is a helper method that makes dealing with storage serialization
// easier for implementors.
// It is designed to be used like:
//
// var st MyStorage
// ret, err := WithStorage(ctx, &st, func() (interface{}, error) {
//   fmt.Println("hey look, my storage is loaded: ", st)
//   return st.Thing, nil
// })
//
// Note that if 'f' returns an error, modifications to the storage are not
// saved.
func WithStorage(ctx *VMContext, st interface{}, f func() (interface{}, error)) (interface{}, error) {
	if err := UnmarshalStorage(ctx.ReadStorage(), st); err != nil {
		return nil, err
	}

	ret, err := f()
	if err != nil {
		return nil, err
	}

	data, err := MarshalStorage(st)
	if err != nil {
		return nil, err
	}

	if err := ctx.WriteStorage(data); err != nil {
		return nil, err
	}

	return ret, nil
}
