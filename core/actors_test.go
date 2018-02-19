package core

import (
	"fmt"
	"math/big"
	"testing"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/assert"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestLoadCodeSuccess(t *testing.T) {
	assert := assert.New(t)

	ex, err := LoadCode(types.TokenActorCid)
	assert.NoError(err)
	assert.Equal(ex, &TokenActor{})
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
	return nil, 2, fmt.Errorf("fail")
}

func NewMockActor(list Exports) *MockActor {
	return &MockActor{
		exports: list,
	}
}

func TestMakeTypedExportSuccess(t *testing.T) {
	t.Run("no return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"two": &FunctionSignature{
				Params: []interface{}{},
				Return: nil,
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "two")(nil)

		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		assert.Nil(ret)
	})

	t.Run("with return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"four": &FunctionSignature{
				Params: []interface{}{},
				Return: []byte{},
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "four")(nil)

		assert.NoError(err)
		assert.Equal(exitCode, uint8(0))
		assert.Equal(string(ret), "hello")
	})

	t.Run("with error return", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"five": &FunctionSignature{
				Params: []interface{}{},
				Return: []byte{},
			},
		})

		ret, exitCode, err := MakeTypedExport(a, "five")(nil)

		assert.Equal(err.Error(), "fail")
		assert.Equal(exitCode, uint8(2))
		assert.Nil(ret)
	})
}

func TestMakeTypedExportFail(t *testing.T) {
	t.Run("missing method on actor", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"one": &FunctionSignature{
				Params: []interface{}{},
				Return: nil,
			},
			"other": &FunctionSignature{
				Params: []interface{}{},
				Return: nil,
			},
		})
		assert.PanicsWithValue(
			"MakeTypedExport could not find passed in method in actor: other",
			func() { MakeTypedExport(a, "other") },
		)
	})

	t.Run("missing method on exports", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(nil)
		assert.PanicsWithValue(
			"MakeTypedExport could not find passed in method in exports: one",
			func() { MakeTypedExport(a, "one") },
		)
	})

	t.Run("too little params", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"one": &FunctionSignature{
				Params: []interface{}{},
				Return: nil,
			},
		})
		assert.PanicsWithValue(
			"MakeTypedExport must receive a function with at least 2 parameters for one",
			func() { MakeTypedExport(a, "one") },
		)
	})

	t.Run("too little return parameters", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"three": &FunctionSignature{
				Params: []interface{}{},
				Return: nil,
			},
		})
		assert.PanicsWithValue(
			"MakeTypedExport must receive a function that returns (uint8, error) for three",
			func() { MakeTypedExport(a, "three") },
		)
	})

	t.Run("wrong return parameters", func(t *testing.T) {
		assert := assert.New(t)

		a := NewMockActor(map[string]*FunctionSignature{
			"two": &FunctionSignature{
				Params: []interface{}{},
				Return: []byte{},
			},
		})
		assert.PanicsWithValue(
			"MakeTypedExport must receive a function that returns ([]uint8, uint8, error) for two",
			func() { MakeTypedExport(a, "two") },
		)
	})
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
