package types

import (
	"bytes"
	"testing"

	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestRoundtrip(t *testing.T) {
	tf.UnitTest(t)

	cases := [][]byte{nil, {}, []byte("bytes")}
	for _, c := range cases {
		b, err := cbor.WrapObject(c, DefaultHashFunction, -1)
		assert.NoError(t, err)
		var out []byte
		err = encoding.Decode(b.RawData(), &out)
		assert.NoError(t, err)
		switch {
		case c == nil:
			assert.Nil(t, out)
		default:
			assert.True(t, bytes.Equal(c, out))
		}
	}
}
