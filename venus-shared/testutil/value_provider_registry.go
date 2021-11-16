package testutil

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

var typeT = reflect.TypeOf((*testing.T)(nil))

func Provide(t *testing.T, dst interface{}, specifiedFns ...interface{}) {
	rval := reflect.ValueOf(dst)
	if kind := rval.Kind(); kind != reflect.Ptr {
		t.Fatalf("value provider can only be applied on to poniters, got %T", dst)
	}

	reg := defaultValueProviderRegistry
	if len(specifiedFns) > 0 {
		reg = defaultValueProviderRegistry.clone()
		for fni := range specifiedFns {
			if err := reg.register(specifiedFns[fni]); err != nil {
				t.Fatalf("register specified provider %T for %T: %s", specifiedFns[fni], dst, err)
			}
		}
	}

	reg.provide(t, rval.Elem())
}

func MustRegisterDefaultValueProvier(fn interface{}) {
	if err := RegisterDefaultValueProvier(fn); err != nil {
		panic(fmt.Errorf("register default value provider %T: %w", fn, err))
	}
}

func RegisterDefaultValueProvier(fn interface{}) error {
	return defaultValueProviderRegistry.register(fn)
}

var defaultValueProviderRegistry = &valueProviderRegistry{
	providers: map[reflect.Type]reflect.Value{},
}

type valueProviderRegistry struct {
	sync.RWMutex
	providers map[reflect.Type]reflect.Value
}

func (r *valueProviderRegistry) clone() *valueProviderRegistry {
	cloned := &valueProviderRegistry{
		providers: map[reflect.Type]reflect.Value{},
	}

	r.Lock()
	for rt, rv := range r.providers {
		cloned.providers[rt] = rv
	}
	r.Unlock()

	return cloned
}

func (r *valueProviderRegistry) register(fn interface{}) error {
	rval := reflect.ValueOf(fn)
	rtyp := rval.Type()

	if rkind := rtyp.Kind(); rkind != reflect.Func {
		return fmt.Errorf("expected provider func, got %s", rkind)
	}

	if numIn := rtyp.NumIn(); numIn != 1 {
		return fmt.Errorf("expected provider func with 1 in, got %d", numIn)
	}

	if numOut := rtyp.NumOut(); numOut != 1 {
		return fmt.Errorf("expected provider func with 1 out, got %d", numOut)
	}

	if inTyp := rtyp.In(0); inTyp != typeT {
		return fmt.Errorf("expected provider's in type to be *testing.T, got %s", inTyp)
	}

	outTyp := rtyp.Out(0)
	r.Lock()
	r.providers[outTyp] = rval
	r.Unlock()

	return nil
}

func (r *valueProviderRegistry) has(want reflect.Type) bool {
	r.RLock()
	_, has := r.providers[want]
	r.RUnlock()

	return has
}

func (r *valueProviderRegistry) provide(t *testing.T, rval reflect.Value) {
	rtyp := rval.Type()
	if !rval.CanSet() {
		return
	}

	r.RLock()
	provider, ok := r.providers[rtyp]
	r.RUnlock()
	if ok {
		ret := provider.Call([]reflect.Value{reflect.ValueOf(t)})
		rval.Set(ret[0])
		return
	}

	r.RLock()
	var convertor reflect.Value
	for pt := range r.providers {
		if pt.ConvertibleTo(rtyp) {
			convertor = r.providers[pt]
			break
		}
	}
	r.RUnlock()

	if convertor.IsValid() {
		ret := convertor.Call([]reflect.Value{reflect.ValueOf(t)})
		rval.Set(ret[0].Convert(rtyp))
		return
	}

	rkind := rtyp.Kind()
	switch rkind {
	case reflect.Slice:
		if rval.IsNil() || rval.Len() == 0 {
			rval.Set(reflect.MakeSlice(rtyp, 1, 1))
		}

		for i := 0; i < rval.Len(); i++ {
			r.provide(t, rval.Index(i))
		}

		return

	case reflect.Array:
		for i := 0; i < rval.Len(); i++ {
			r.provide(t, rval.Index(i))
		}

		return

	case reflect.Ptr:
		if rval.IsNil() {
			rval.Set(reflect.New(rtyp.Elem()))
		}

		r.provide(t, rval.Elem())
		return

	case reflect.Struct:
		for i := 0; i < rval.NumField(); i++ {
			fieldVal := rval.Field(i)
			r.provide(t, fieldVal)
		}

		return
	}

}
