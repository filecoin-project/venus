package actor_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	. "github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/vmcontext"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
)

func TestActorCid(t *testing.T) {
	tf.UnitTest(t)

	actor1 := NewActor(types.AccountActorCodeCid, types.ZeroAttoFIL)
	actor2 := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))
	actor2.Head = requireCid(t, "Actor 2 State")
	actor1.IncNonce()

	c1, err := actor1.Cid()
	assert.NoError(t, err)
	c2, err := actor2.Cid()
	assert.NoError(t, err)

	assert.NotEqual(t, c1.String(), c2.String())
}

func TestActorFormat(t *testing.T) {
	tf.UnitTest(t)

	accountActor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(t, formatted, "AccountActor")
	assert.Contains(t, formatted, "balance: 5")
	assert.Contains(t, formatted, "nonce: 0")

	minerActor := NewActor(types.MinerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(t, formatted, "MinerActor")

	storageMarketActor := NewActor(types.StorageMarketActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(t, formatted, "StorageMarketActor")

	paymentBrokerActor := NewActor(types.PaymentBrokerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", paymentBrokerActor)
	assert.Contains(t, formatted, "PaymentBrokerActor")
}

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	cid, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return cid
}

type MockActor struct {
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

func NewMockActor(list dispatch.Exports) *MockActor {
	return &MockActor{
		signatures: list,
	}
}

var _ dispatch.ExecutableActor = (*MockActor)(nil)

// Method returns method definition for a given method id.
func (a *MockActor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
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

func (a *MockActor) InitializeState(storage runtime.Storage, initializerData interface{}) error {
	return nil
}

type impl MockActor

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

func makeCtx(method types.MethodID) runtime.Runtime {
	addrGetter := address.NewForTestGetter()

	vmCtxParams := vmcontext.NewContextParams{
		Message:     types.NewUnsignedMessage(addrGetter(), addrGetter(), 0, types.ZeroAttoFIL, method, nil),
		GasTracker:  gastracker.NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
		Actors:      builtin.DefaultActors,
	}

	return vmcontext.NewVMContext(vmCtxParams)
}

func TestMakeTypedExportSuccess(t *testing.T) {
	tf.UnitTest(t)

	t.Run("no return", func(t *testing.T) {
		a := NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Two: {
				Params: nil,
				Return: nil,
			},
		})

		fn, ok := MakeTypedExport(a, Two)
		require.True(t, ok)

		ret, exitCode, err := fn(makeCtx(Two))
		assert.NoError(t, err)
		assert.Equal(t, exitCode, uint8(0))
		assert.Nil(t, ret)
	})

	t.Run("with return", func(t *testing.T) {
		a := NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Four: {
				Params: nil,
				Return: []abi.Type{abi.Bytes},
			},
		})

		fn, ok := MakeTypedExport(a, Four)
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
		a := NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Five: {
				Params: []abi.Type{},
				Return: []abi.Type{abi.Bytes},
			},
		})

		fn, ok := MakeTypedExport(a, Five)
		require.True(t, ok)

		ret, exitCode, err := fn(makeCtx(Five))
		assert.Contains(t, err.Error(), "fail5")
		assert.Equal(t, exitCode, uint8(2))
		assert.Nil(t, ret)
	})

	t.Run("with error that is not revert or fault", func(t *testing.T) {
		a := NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
			Six: {
				Params: nil,
				Return: nil,
			},
		})

		fn, ok := MakeTypedExport(a, Six)
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
		Actor  *MockActor
		Method types.MethodID
		Error  string
		Panics bool
	}{
		{
			Name: "missing method on actor",
			Actor: NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
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
			Error:  "MakeTypedExport could not find passed in method in actor",
		},
		{
			Name: "too little params",
			Actor: NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				One: {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (runtime.Runtime) (uint8, error), but got: func() (uint8, error)",
			Method: One,
		},
		{
			Name: "too little return parameters",
			Actor: NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Three: {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (runtime.Runtime) (uint8, error), but got: func(runtime.Runtime) error",
			Method: Three,
		},
		{
			Name: "wrong return parameters",
			Actor: NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Two: {
					Params: nil,
					Return: []abi.Type{abi.Bytes},
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (runtime.Runtime) ([]byte, uint8, error), but got: func(runtime.Runtime) (uint8, error)",
			Method: Two,
		},
		{
			Name: "multiple return parameters",
			Actor: NewMockActor(map[types.MethodID]*dispatch.FunctionSignature{
				Two: {
					Params: nil,
					Return: []abi.Type{abi.Bytes, abi.Bytes},
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (runtime.Runtime) ([]byte, []byte, uint8, error), but got: func(runtime.Runtime) (uint8, error)",
			Method: Two,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert.PanicsWithValue(t, tc.Error, func() {
				_, ok := MakeTypedExport(tc.Actor, tc.Method)
				if !ok {
					panic("MakeTypedExport could not find passed in method in actor")
				}
			})
		})
	}
}
