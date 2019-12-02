package vmcontext

import (
	"fmt"
	"reflect"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeTypedExportSuccess(t *testing.T) {
	tf.UnitTest(t)

	t.Run("no return", func(t *testing.T) {
		a := newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Two: {
				Params: nil,
				Return: nil,
			},
		})

		fn, ok := makeTypedExport(a, Two)
		require.True(t, ok)

		ret, exitCode, err := fn(makeCtx(Two))
		assert.NoError(t, err)
		assert.Equal(t, exitCode, uint8(0))
		assert.Nil(t, ret)
	})

	t.Run("with return", func(t *testing.T) {
		a := newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Four: {
				Params: nil,
				Return: []abi.Type{abi.Bytes},
			},
		})

		fn, ok := makeTypedExport(a, Four)
		require.True(t, ok)

		ret, exitCode, err := fn(makeCtx(Four))
		assert.NoError(t, err)
		assert.Equal(t, exitCode, uint8(0))

		vv, err := abi.DecodeValues(ret, a.signatures[Four].Return)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(vv))
		assert.Equal(t, vv[0].Val, []byte("hello"))
	})

	t.Run("with error return", func(t *testing.T) {
		a := newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Five: {
				Params: []abi.Type{},
				Return: []abi.Type{abi.Bytes},
			},
		})

		fn, ok := makeTypedExport(a, Five)
		require.True(t, ok)

		ret, exitCode, err := fn(makeCtx(Five))
		assert.Contains(t, err.Error(), "fail5")
		assert.Equal(t, exitCode, uint8(2))
		assert.Nil(t, ret)
	})

	t.Run("with error that is not revert or fault", func(t *testing.T) {
		a := newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Six: {
				Params: nil,
				Return: nil,
			},
		})

		fn, ok := makeTypedExport(a, Six)
		require.True(t, ok)

		assert.Panics(t, func() {
			_, _, _ = fn(makeCtx(Six))
		})
	})
}

func TestMakeTypedExportFail(t *testing.T) {
	tf.UnitTest(t)
	otherID := types.MethodID(8276363)

	testCases := []struct {
		Name   string
		Actor  *mockActor
		Method types.MethodID
		Error  string
		Panics bool
	}{
		{
			Name: "missing method on actor",
			Actor: newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				One: {
					Params: nil,
					Return: nil,
				},
				// unregistered id
				otherID: {
					Params: nil,
					Return: nil,
				},
			}),
			Method: otherID,
			Error:  "makeTypedExport could not find passed in method in actor",
		},
		{
			Name: "too little params",
			Actor: newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				One: {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "makeTypedExport must receive a function with signature: func (runtime.Runtime) (uint8, error), but got: func() (uint8, error)",
			Method: One,
		},
		{
			Name: "too little return parameters",
			Actor: newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Three: {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "makeTypedExport must receive a function with signature: func (runtime.Runtime) (uint8, error), but got: func(runtime.Runtime) error",
			Method: Three,
		},
		{
			Name: "wrong return parameters",
			Actor: newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Two: {
					Params: nil,
					Return: []abi.Type{abi.Bytes},
				},
			}),
			Error:  "makeTypedExport must receive a function with signature: func (runtime.Runtime) ([]byte, uint8, error), but got: func(runtime.Runtime) (uint8, error)",
			Method: Two,
		},
		{
			Name: "multiple return parameters",
			Actor: newMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Two: {
					Params: nil,
					Return: []abi.Type{abi.Bytes, abi.Bytes},
				},
			}),
			Error:  "makeTypedExport must receive a function with signature: func (runtime.Runtime) ([]byte, []byte, uint8, error), but got: func(runtime.Runtime) (uint8, error)",
			Method: Two,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert.PanicsWithValue(t, tc.Error, func() {
				_, ok := makeTypedExport(tc.Actor, tc.Method)
				if !ok {
					panic("makeTypedExport could not find passed in method in actor")
				}
			})
		})
	}
}

type mockActor struct {
	signatures dispatch.Exports
}

const (
	One types.MethodID = iota + 32
	Two
	Three
	Four
	Five
	Six
)

func newMockActor(list dispatch.Exports) *mockActor {
	return &mockActor{
		signatures: list,
	}
}

var _ dispatch.ExecutableActor = (*mockActor)(nil)

// Method returns method definition for a given method id.
func (a *mockActor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	signature, ok := a.signatures[id]
	if !ok {
		return nil, nil, false
	}
	switch id {
	case One:
		return reflect.ValueOf((*impl)(a).one), signature, true
	case Two:
		return reflect.ValueOf((*impl)(a).two), signature, true
	case Three:
		return reflect.ValueOf((*impl)(a).three), signature, true
	case Four:
		return reflect.ValueOf((*impl)(a).four), signature, true
	case Five:
		return reflect.ValueOf((*impl)(a).five), signature, true
	case Six:
		return reflect.ValueOf((*impl)(a).six), signature, true
	default:
		return nil, nil, false
	}
}

func (a *mockActor) InitializeState(storage runtime.Storage, initializerData interface{}) error {
	return nil
}

type impl mockActor

func (*impl) one() (uint8, error) {
	return 0, nil
}

func (*impl) two(ctx runtime.Runtime) (uint8, error) {
	return 0, nil
}

func (*impl) three(ctx runtime.Runtime) error {
	return nil
}

func (*impl) four(ctx runtime.Runtime) ([]byte, uint8, error) {
	return []byte("hello"), 0, nil
}

func (*impl) five(ctx runtime.Runtime) ([]byte, uint8, error) {
	return nil, 2, errors.NewRevertError("fail5")
}

func (*impl) six(ctx runtime.Runtime) (uint8, error) {
	return 0, fmt.Errorf("NOT A REVERT OR FAULT -- PROGRAMMER ERROR")
}

func makeCtx(method types.MethodID) *VMContext {
	addrGetter := address.NewForTestGetter()

	vmCtxParams := NewContextParams{
		Message:     types.NewUnsignedMessage(addrGetter(), addrGetter(), 0, types.ZeroAttoFIL, method, nil),
		GasTracker:  gastracker.NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
		Actors:      builtin.DefaultActors,
		To:          &actor.Actor{},
	}

	return NewVMContext(vmCtxParams)
}
