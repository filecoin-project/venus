package types

import (
	"bytes"
	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoundtrip(t *testing.T) {
	assert := assert.New(t)
	cases := []Bytes{Bytes(nil), {}, Bytes([]byte("bytes"))}
	for _, c := range cases {
		b, err := cbor.WrapObject(c, DefaultHashFunction, -1)
		assert.NoError(err)
		var out Bytes
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
