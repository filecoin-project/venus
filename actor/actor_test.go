package actor_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/abi"
	. "github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

type MockActor struct {
	exports exec.Exports
}

func (a *MockActor) Exports() exec.Exports {
	return a.exports
}

func (a *MockActor) NewStorage() interface{} {
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
	addrGetter := types.NewAddressForTestGetter()
	return vm.NewVMContext(nil, nil, types.NewMessage(addrGetter(), addrGetter(), 0, nil, method, nil), nil, nil)
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
		assert.PanicsWithValue("you are a bad person: error must be either a reverterror or a fault", func() {
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

func TestMarshalValue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert := assert.New(t)

		testCases := []struct {
			In  interface{}
			Out []byte
		}{
			{In: []byte("hello"), Out: []byte("hello")},
			{In: big.NewInt(100), Out: big.NewInt(100).Bytes()},
			{In: "hello", Out: []byte("hello")},
		}

		for _, tc := range testCases {
			out, err := MarshalValue(tc.In)
			assert.NoError(err)
			assert.Equal(out, tc.Out)
		}
	})

	t.Run("failure", func(t *testing.T) {
		assert := assert.New(t)

		out, err := MarshalValue(big.NewRat(1, 2))
		assert.Equal(err.Error(), "unknown type: *big.Rat")
		assert.Nil(out)
	})
}
