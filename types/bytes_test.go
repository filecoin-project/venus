package types

import (
	"bytes"
	cbor "github.com/ipfs/go-ipld-cbor"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	cases := [][]byte{nil, {}, []byte("bytes")}
	for _, c := range cases {
		b, err := cbor.WrapObject(c, DefaultHashFunction, -1)
		assert.NoError(t, err)
		var out []byte
		err = cbor.DecodeInto(b.RawData(), &out)
		assert.NoError(t, err)
		switch {
		case c == nil:
			assert.Nil(t, out)
		default:
			assert.True(t, bytes.Equal(c, out))
		}
	}
}
