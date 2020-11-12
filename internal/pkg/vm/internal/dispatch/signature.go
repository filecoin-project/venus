package dispatch

import (
	"bytes"
	"reflect"

	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// MethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type MethodSignature interface {
	// ArgNil returns a nil interface for the typed argument expected by the actor method.
	ArgNil() reflect.Value
	// ArgInterface returns the typed argument expected by the actor method.
	ArgInterface(argBytes []byte) (interface{}, error)
	// ReturnInterface returns the methods typed return.
	ReturnInterface(returnBytes []byte) (interface{}, error)
}

type methodSignature struct {
	method method
}

var _ MethodSignature = (*methodSignature)(nil)

func (ms *methodSignature) ArgNil() reflect.Value {
	t := ms.method.Type().In(1)
	v := reflect.New(t)
	return v.Elem()
}

func (ms *methodSignature) ArgInterface(argBytes []byte) (interface{}, error) {
	// decode arg1 (this is the payload for the actor method)
	t := ms.method.Type().In(1)
	v := reflect.New(t)

	// This would be better fixed in then encoding library.
	obj := v.Elem().Interface()
	if _, ok := obj.(cbg.CBORUnmarshaler); ok {
		buf := bytes.NewBuffer(argBytes)
		auxv := reflect.New(t.Elem())
		obj = auxv.Interface()

		unmarsh := obj.(cbg.CBORUnmarshaler)
		if err := unmarsh.UnmarshalCBOR(buf); err != nil {
			return nil, err
		}
		return unmarsh, nil
	}

	if err := encoding.Decode(argBytes, v.Interface()); err != nil {
		return nil, errors.Wrap(err, "failed to decode bytes as method argument")
	}

	// dereference the extra pointer created by `reflect.New()`
	return v.Elem().Interface(), nil
}

func (ms *methodSignature) ReturnInterface(returnBytes []byte) (interface{}, error) {
	// decode arg1 (this is the payload for the actor method)
	t := ms.method.Type().Out(0)
	v := reflect.New(t)
	if err := encoding.Decode(returnBytes, v.Interface()); err != nil {
		return nil, errors.Wrap(err, "failed to decode return bytes for method")
	}

	return v.Interface(), nil
}
