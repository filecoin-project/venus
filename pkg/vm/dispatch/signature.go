package dispatch

import (
	"bytes"
	"fmt"
	"reflect"

	cbg "github.com/whyrusleeping/cbor-gen"
)

// MethodSignature wraps a specific method and allows you to encode/decodes input/output bytes into concrete types.
type MethodSignature interface {
	// ArgNil returns a nil interface for the typed argument expected by the actor method.
	ArgNil() reflect.Value
	// ArgInterface returns the typed argument expected by the actor method.
	ArgInterface(argBytes []byte) (interface{}, error)
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
	v := reflect.New(t.Elem())
	obj := v.Interface()

	if val, ok := obj.(cbg.CBORUnmarshaler); ok {
		buf := bytes.NewReader(argBytes)
		if err := val.UnmarshalCBOR(buf); err != nil {
			return nil, err
		}
		return val, nil
	}
	return nil, fmt.Errorf("type %T does not implement UnmarshalCBOR", obj)

}
