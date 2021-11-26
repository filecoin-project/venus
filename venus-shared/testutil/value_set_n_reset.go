package testutil

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func ValueSetNReset(t *testing.T, name string, onSet func(), onReset func(), vsets ...interface{}) {
	psize := len(vsets)
	require.Greaterf(t, psize, 0, "value sets should not be empty for case %s", name)
	require.Truef(t, psize%2 == 0, "params count should be odd for case %s", name)

	ptrs := make([]reflect.Value, psize/2)
	originalVals := make([]reflect.Value, psize/2)
	for i := 0; i < psize/2; i++ {
		pi := i * 2
		ptr := reflect.ValueOf(vsets[pi])
		require.Equalf(t, ptr.Type().Kind(), reflect.Ptr, "#%d param should be pointer to the target value for case %s", i, name)
		ptrs[i] = ptr

		originVal := reflect.New(ptr.Elem().Type())
		originVal.Elem().Set(ptr.Elem())

		originalVals[i] = originVal
		ptr.Elem().Set(reflect.ValueOf(vsets[pi+1]))
	}

	if onSet != nil {
		onSet()
	}

	for i := range ptrs {
		ptrs[i].Elem().Set(originalVals[i].Elem())
	}

	if onReset != nil {
		onReset()
	}
}
