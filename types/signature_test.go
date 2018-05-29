package types

import (
	"bytes"
	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundtrip(t *testing.T) {
	assert := assert.New(t)
	cases := []Signature{Signature(nil), {}, Signature([]byte("signature"))}
	for _, c := range cases {
		b, err := cbor.WrapObject(c, DefaultHashFunction, -1)
		assert.NoError(err)
		var out Signature
		err = cbor.DecodeInto(b.RawData(), &out)
		assert.NoError(err)
		switch {
		case c == nil:
			assert.Nil(out)
		default:
			assert.True(bytes.Equal(c, out))
		}
	}
}
