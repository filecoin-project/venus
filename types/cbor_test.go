package types

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssimilatePointer(t *testing.T) {
	testCases := []struct {
		Source interface{}
		Target reflect.Type
		Result interface{}
	}{
		{
			Source: big.NewInt(10),
			Target: reflect.TypeOf(&big.Int{}),
			Result: big.NewInt(10),
		},
		{
			Source: *big.NewInt(10),
			Target: reflect.TypeOf(&big.Int{}),
			Result: big.NewInt(10),
		},
		{
			Source: *big.NewInt(10),
			Target: reflect.TypeOf(big.Int{}),
			Result: *big.NewInt(10),
		},
		{
			Source: big.NewInt(10),
			Target: reflect.TypeOf(big.Int{}),
			Result: *big.NewInt(10),
		},
		{
			Source: big.NewInt(10).Bytes(),
			Target: reflect.TypeOf(&big.Int{}),
			Result: big.NewInt(10),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s->%s", reflect.TypeOf(tc.Source), tc.Target), func(t *testing.T) {
			assert := assert.New(t)

			assert.Equal(
				assimilatePointer(tc.Source, tc.Target).Interface(),
				tc.Result,
			)
		})
	}
}

func TestAssimilateSlice(t *testing.T) {
	testCases := []interface{}{
		[]byte{1, 2, 3},
		[]string{"hello", "world"},
		[]*big.Int{big.NewInt(1), big.NewInt(10), big.NewInt(2)},
	}

	for _, data := range testCases {
		t.Run(reflect.TypeOf(data).String(), func(t *testing.T) {
			assert := assert.New(t)

			assert.Equal(
				assimilateSlice(data, reflect.TypeOf(data)).Interface(),
				data,
			)
		})
	}
}

func TestAssimilateMap(t *testing.T) {
	testCases := []interface{}{
		map[string]string{"hello": "world"},
		map[string]*big.Int{"hello": big.NewInt(10)},
		map[string][]byte{"hello": {1, 2, 3}},
		map[Address]Address{Address("hi"): Address("you")},
	}

	for _, data := range testCases {
		t.Run(reflect.TypeOf(data).String(), func(t *testing.T) {
			assert := assert.New(t)

			assert.Equal(
				assimilateMap(data, reflect.TypeOf(data)).Interface(),
				data,
			)
		})
	}
}
