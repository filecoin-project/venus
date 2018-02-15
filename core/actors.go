package core

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

var BuiltinActors = map[string]ExecutableActor{}

type Exports map[string]*FunctionSignature

type ExecutableActor interface {
	Exports() Exports
}

type ExportFunc func(ctx *VMContext) ([]byte, uint8, error)

// TODO: convert signatures into non go types, but rather low level agreed up types
type FunctionSignature struct {
	Params []interface{}
	Return interface{}
}

func init() {
	BuiltinActors[types.AccountActorCid.KeyString()] = &AccountActor{}
}

// LoadCode fetches the code referenced by the passed in CID.
func LoadCode(code *cid.Cid) (ExecutableActor, error) {
	if code == nil {
		return nil, fmt.Errorf("missing code: %s", code)
	}
	actor, ok := BuiltinActors[code.KeyString()]
	if !ok {
		return nil, fmt.Errorf("unknown code")
	}

	return actor, nil
}

func MakeTypedExport(actor ExecutableActor, method string) ExportFunc {
	f, ok := reflect.TypeOf(actor).MethodByName(strings.Title(method))
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in actor: %s", method))
	}

	exports := actor.Exports()
	signature, ok := exports[method]
	if !ok {
		panic(fmt.Sprintf("MakeTypedExport could not find passed in method in params: %s", method))
	}

	val := f.Func
	t := f.Type
	// number of input args, struct receiver + context + dynamic params
	numIn := 2 + len(signature.Params)

	if t.Kind() != reflect.Func || t.NumIn() != numIn {
		fmt.Println(t.Kind())
		panic(fmt.Sprintf("MakeTypedExport must receive a function with at least %d parameters for %s", numIn, method))
	}

	exitType := reflect.Uint8
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	if signature.Return != nil {
		retType := reflect.TypeOf(signature.Return)

		if t.NumOut() != 3 || t.Out(0) != retType || t.Out(1).Kind() != exitType || !t.Out(2).Implements(errorType) {
			panic(fmt.Sprintf("MakeTypedExport must receive a function that returns (%suint8, error) for %s", retType, method))
		}
	} else {
		if t.NumOut() != 2 || t.Out(0).Kind() != exitType || !t.Out(1).Implements(errorType) {
			panic(fmt.Sprintf("MakeTypedExport must receive a function that returns (uint8, error) for %s", method))
		}
	}

	return func(ctx *VMContext) ([]byte, uint8, error) {
		args := []reflect.Value{
			reflect.ValueOf(actor),
			reflect.ValueOf(ctx),
		}

		for i, paramType := range signature.Params {
			actualParam := ctx.Message().Params()[i]

			tActual := reflect.TypeOf(actualParam)
			tExpected := reflect.TypeOf(paramType)

			if tActual != tExpected {
				return nil, 0, fmt.Errorf("invalid params type: expected %v, got %v", tExpected, tActual)
			}
			args = append(args, reflect.ValueOf(actualParam))
		}

		out := val.Call(args)

		if signature.Return != nil {
			ret, retErr := marshalValue(out[0].Interface())
			exitCode, ok := out[1].Interface().(uint8)
			if !ok {
				panic("invalid return value")
			}
			err, ok := out[2].Interface().(error)
			if !ok {
				err = nil
			}

			if retErr != nil {
				if err != nil {
					err = errors.Wrap(err, retErr.Error())
				} else {
					err = retErr
				}
			}

			return ret, exitCode, err
		} else {
			exitCode, ok := out[0].Interface().(uint8)
			if !ok {
				panic("invalid return value")
			}
			err, ok := out[1].Interface().(error)
			if !ok {
				err = nil
			}

			return nil, exitCode, err
		}
	}
}

// marshalValue serializes a given go type into a byte slice.
// The returned format matches the format that is expected to be interoperapble between VM and
// the rest of the system.
func marshalValue(val interface{}) ([]byte, error) {
	switch t := val.(type) {
	case *big.Int:
		return val.(*big.Int).Bytes(), nil
	default:
		return nil, fmt.Errorf("unknown type: %s", t)
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
