package actor_test

import (
	"fmt"
	"testing"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	. "github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
)

func TestActorCid(t *testing.T) {
	assert := assert.New(t)

	actor1 := NewActor(types.AccountActorCodeCid, nil)
	actor2 := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))
	actor2.Head = requireCid(t, "Actor 2 State")
	actor1.IncNonce()

	c1, err := actor1.Cid()
	assert.NoError(err)
	c2, err := actor2.Cid()
	assert.NoError(err)

	assert.NotEqual(c1.String(), c2.String())
}

func TestActorFormat(t *testing.T) {
	assert := assert.New(t)
	accountActor := NewActor(types.AccountActorCodeCid, types.NewAttoFILFromFIL(5))

	formatted := fmt.Sprintf("%v", accountActor)
	assert.Contains(formatted, "AccountActor")
	assert.Contains(formatted, "balance: 5")
	assert.Contains(formatted, "nonce: 0")

	minerActor := NewActor(types.MinerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", minerActor)
	assert.Contains(formatted, "MinerActor")

	storageMarketActor := NewActor(types.StorageMarketActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", storageMarketActor)
	assert.Contains(formatted, "StorageMarketActor")

	paymentBrokerActor := NewActor(types.PaymentBrokerActorCodeCid, types.NewAttoFILFromFIL(5))
	formatted = fmt.Sprintf("%v", paymentBrokerActor)
	assert.Contains(formatted, "PaymentBrokerActor")
}

func requireCid(t *testing.T, data string) cid.Cid {
	prefix := cid.V1Builder{Codec: cid.Raw, MhType: types.DefaultHashFunction}
	cid, err := prefix.Sum([]byte(data))
	require.NoError(t, err)
	return cid
}

type MockActor struct {
	exports exec.Exports
}

func (a *MockActor) Exports() exec.Exports {
	return a.exports
}

func (a *MockActor) InitializeState(storage exec.Storage, initializerData interface{}) error {
	return nil
}

func (a *MockActor) One() (uint8, error) {
	return 0, nil
}

func (a *MockActor) Two(ctx exec.VMContext) (uint8, error) {
	return 0, nil
}

func (a *MockActor) Three(ctx exec.VMContext) error {
	return nil
}

func (a *MockActor) Four(ctx exec.VMContext) ([]byte, uint8, error) {
	return []byte("hello"), 0, nil
}

func (a *MockActor) Five(ctx exec.VMContext) ([]byte, uint8, error) {
	return nil, 2, errors.NewRevertError("fail5")
}

func (a *MockActor) Six(ctx exec.VMContext) (uint8, error) {
	return 0, fmt.Errorf("NOT A REVERT OR FAULT -- PROGRAMMER ERROR")
}

func NewMockActor(list exec.Exports) *MockActor {
	return &MockActor{
		exports: list,
	}
}

func makeCtx(method string) exec.VMContext {
	addrGetter := address.NewForTestGetter()

	vmCtxParams := vm.NewContextParams{
		Message:     types.NewMessage(addrGetter(), addrGetter(), 0, nil, method, nil),
		GasTracker:  vm.NewGasTracker(),
		BlockHeight: types.NewBlockHeight(0),
	}

	return vm.NewVMContext(vmCtxParams)
}

func TestMakeTypedExportSuccess(t *testing.T) {
	t.Run("no return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*exec.FunctionSignature{
			"two": {
				Params: nil,
				Return: nil,
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "two")(makeCtx("two"))

		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		assert.Nil(ret)
	})

	t.Run("with return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*exec.FunctionSignature{
			"four": {
				Params: nil,
				Return: []abi.Type{abi.Bytes},
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "four")(makeCtx("four"))

		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		vv, err := abi.DecodeValues(ret, a.Exports()["four"].Return)
		assert.NoError(err)
		assert.Equal(1, len(vv))
		assert.Equal(vv[0].Val, []byte("hello"))
	})

	t.Run("with error return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*exec.FunctionSignature{
			"five": {
				Params: []abi.Type{},
				Return: []abi.Type{abi.Bytes},
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "five")(makeCtx("five"))

		assert.Contains(err.Error(), "fail5")
		assert.Equal(exitCode, uint8(2))
		assert.Nil(ret)
	})

	t.Run("with error that is not revert or fault", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*exec.FunctionSignature{
			"six": {
				Params: nil,
				Return: nil,
			},
		})

		exportedFunc := MakeTypedExport(a, "six")
		assert.Panics(func() {
			exportedFunc(makeCtx("six"))
		})
	})
}

func TestMakeTypedExportFail(t *testing.T) {
	testCases := []struct {
		Name   string
		Actor  *MockActor
		Method string
		Error  string
	}{
		{
			Name: "missing method on actor",
			Actor: NewMockActor(map[string]*exec.FunctionSignature{
				"one": {
					Params: nil,
					Return: nil,
				},
				"other": {
					Params: nil,
					Return: nil,
				},
			}),
			Method: "other",
			Error:  "MakeTypedExport could not find passed in method in actor: other",
		},
		{
			Name:   "missing method on exports",
			Actor:  NewMockActor(nil),
			Error:  "MakeTypedExport could not find passed in method in exports: one",
			Method: "one",
		},
		{
			Name: "too little params",
			Actor: NewMockActor(map[string]*exec.FunctionSignature{
				"one": {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (Actor, exec.VMContext) (uint8, error), but got: func(*actor_test.MockActor) (uint8, error)",
			Method: "one",
		},
		{
			Name: "too little return parameters",
			Actor: NewMockActor(map[string]*exec.FunctionSignature{
				"three": {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (Actor, exec.VMContext) (uint8, error), but got: func(*actor_test.MockActor, exec.VMContext) error",
			Method: "three",
		},
		{
			Name: "wrong return parameters",
			Actor: NewMockActor(map[string]*exec.FunctionSignature{
				"two": {
					Params: nil,
					Return: []abi.Type{abi.Bytes},
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (Actor, exec.VMContext) ([]byte, uint8, error), but got: func(*actor_test.MockActor, exec.VMContext) (uint8, error)",
			Method: "two",
		},
		{
			Name: "multiple return parameters",
			Actor: NewMockActor(map[string]*exec.FunctionSignature{
				"two": {
					Params: nil,
					Return: []abi.Type{abi.Bytes, abi.Bytes},
				},
			}),
			Error:  "MakeTypedExport must receive a function with signature: func (Actor, exec.VMContext) ([]byte, []byte, uint8, error), but got: func(*actor_test.MockActor, exec.VMContext) (uint8, error)",
			Method: "two",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)

			assert.PanicsWithValue(tc.Error, func() {
				MakeTypedExport(tc.Actor, tc.Method)
			})
		})
	}
}
