package abi

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// TODO: tests that check the exact serialization of different inputs.
// defer this until we're reasonably unlikely to change the way we serialize
// things.

func TestBasicEncodingRoundTrip(t *testing.T) {
	tf.UnitTest(t)

	addrGetter := address.NewForTestGetter()

	cases := map[string][]interface{}{
		"empty":      nil,
		"one-int":    {big.NewInt(579)},
		"one addr":   {addrGetter()},
		"two addrs":  {addrGetter(), addrGetter()},
		"one []byte": {[]byte("foo")},
		"two []byte": {[]byte("foo"), []byte("bar")},
		"a string":   {"flugzeug"},
		"mixed":      {big.NewInt(17), []byte("beep"), "mr rogers", addrGetter()},
		"sector ids": {uint64(1234), uint64(0)},
		"predicate": {
			&types.Predicate{
				To:     addrGetter(),
				Method: types.MethodID(82),
				Params: []interface{}{uint64(3), []byte("proof")},
			},
		},
		"miner post states": {
			&map[string]uint64{address.TestAddress.String(): 1, address.TestAddress2.String(): 2},
		},
	}

	for tname, tcase := range cases {
		t.Run(tname, func(t *testing.T) {
			vals, err := ToValues(tcase)
			assert.NoError(t, err)

			data, err := EncodeValues(vals)
			assert.NoError(t, err)

			var types []Type
			for _, val := range vals {
				types = append(types, val.Type)
			}

			outVals, err := DecodeValues(data, types)
			assert.NoError(t, err)
			assert.Equal(t, vals, outVals)

			assert.Equal(t, tcase, FromValues(outVals))
		})
	}
}

type fooTestStruct struct {
	Bar string
	Baz uint64
}

func TestToValuesFailures(t *testing.T) {
	tf.UnitTest(t)

	cases := []struct {
		name   string
		vals   []interface{}
		expErr string
	}{
		{
			name:   "nil value",
			vals:   []interface{}{nil},
			expErr: "unsupported type: <nil>",
		},
		{
			name:   "normal int",
			vals:   []interface{}{17},
			expErr: "unsupported type: int",
		},
		{
			name:   "a struct",
			vals:   []interface{}{&fooTestStruct{"b", 99}},
			expErr: "unsupported type: *abi.fooTestStruct",
		},
	}

	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			_, err := ToValues(tcase.vals)
			assert.EqualError(t, err, tcase.expErr)
		})
	}
}
