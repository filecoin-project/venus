package core

import (
	"fmt"
	"math/big"
	"testing"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestLoadCodeSuccess(t *testing.T) {
	assert := assert.New(t)

	ex, err := LoadCode(types.AccountActorCodeCid)
	assert.NoError(err)
	assert.Equal(ex, &AccountActor{})
}

func TestLoadCodeFail(t *testing.T) {
	assert := assert.New(t)

	missingCid, err := cidFromString("missing")
	assert.NoError(err)

	ex, err := LoadCode(missingCid)
	assert.Equal(err.Error(), fmt.Sprintf("unknown code: %s", missingCid.String()))
	assert.Nil(ex)

	ex, err = LoadCode(nil)
	assert.Equal(err.Error(), "missing code")
	assert.Nil(ex)
}

type MockActor struct {
	exports Exports
}

func (a *MockActor) Exports() Exports {
	return a.exports
}
func (a *MockActor) NewStorage() interface{} {
	return nil
}

func (a *MockActor) One() (uint8, error) {
	return 0, nil
}

func (a *MockActor) Two(ctx *VMContext) (uint8, error) {
	return 0, nil
}

func (a *MockActor) Three(ctx *VMContext) error {
	return nil
}

func (a *MockActor) Four(ctx *VMContext) ([]byte, uint8, error) {
	return []byte("hello"), 0, nil
}

func (a *MockActor) Five(ctx *VMContext) ([]byte, uint8, error) {
	return nil, 2, newRevertError("fail5")
}

func (a *MockActor) Six(ctx *VMContext) (uint8, error) {
	return 0, fmt.Errorf("NOT A REVERT OR FAULT -- PROGRAMMER ERROR")
}

func NewMockActor(list Exports) *MockActor {
	return &MockActor{
		exports: list,
	}
}

func makeCtx(method string) *VMContext {
	addrGetter := types.NewAddressForTestGetter()
	return NewVMContext(nil, nil, types.NewMessage(addrGetter(), addrGetter(), nil, method, nil), nil)
}

func TestMakeTypedExportSuccess(t *testing.T) {
	t.Run("no return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
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

		a := NewMockActor(map[string]*FunctionSignature{
			"four": {
				Params: nil,
				Return: []abi.Type{abi.Bytes},
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "four")(makeCtx("four"))

		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		assert.Equal(string(ret), "hello")
	})

	t.Run("with error return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
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

		a := NewMockActor(map[string]*FunctionSignature{
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
			Actor: NewMockActor(map[string]*FunctionSignature{
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
			Actor: NewMockActor(map[string]*FunctionSignature{
				"one": {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function with 2 parameters for one",
			Method: "one",
		},
		{
			Name: "too little return parameters",
			Actor: NewMockActor(map[string]*FunctionSignature{
				"three": {
					Params: nil,
					Return: nil,
				},
			}),
			Error:  "MakeTypedExport must receive a function that returns (uint8, error) for three",
			Method: "three",
		},
		{
			Name: "wrong return parameters",
			Actor: NewMockActor(map[string]*FunctionSignature{
				"two": {
					Params: nil,
					Return: []abi.Type{abi.Bytes},
				},
			}),
			Error:  "MakeTypedExport must receive a function that returns ([]byte, uint8, error) for two",
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
			out, err := marshalValue(tc.In)
			assert.NoError(err)
			assert.Equal(out, tc.Out)
		}
	})

	t.Run("failure", func(t *testing.T) {
		assert := assert.New(t)

		out, err := marshalValue(big.NewRat(1, 2))
		assert.Equal(err.Error(), "unknown type: *big.Rat")
		assert.Nil(out)
	})
}

func cidFromString(input string) (*cid.Cid, error) {
	prefix := cid.NewPrefixV1(cid.DagCBOR, types.DefaultHashFunction)
	return prefix.Sum([]byte(input))
}
