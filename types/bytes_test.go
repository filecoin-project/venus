package types

import (
	"bytes"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestRoundtrip(t *testing.T) {
	assert := assert.New(t)
	cases := [][]byte{nil, {}, []byte("bytes")}
	for _, c := range cases {
		b, err := cbor.WrapObject(c, DefaultHashFunction, -1)
		assert.NoError(err)
		var out []byte
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
