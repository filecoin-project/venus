package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "github.com/ipfs/go-ipld-cbor"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestChannelIDCreation(t *testing.T) {
	tf.UnitTest(t)

	a := NewChannelID(123)
	assert.IsType(t, &ChannelID{}, a)

	ab := a.Bytes()
	b := NewChannelIDFromBytes(ab)
	assert.Equal(t, a, b)

	as := a.String()
	assert.Equal(t, as, "123")
	c, ok := NewChannelIDFromString(as, 10)
	assert.True(t, ok)
	assert.Equal(t, a, c)

	next := a.Inc()
	assert.Equal(t, "124", next.String())

	_, ok = NewChannelIDFromString("asdf", 10)
	assert.False(t, ok)
}

func TestChannelIDCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(ChannelID)) == identity(ChannelID)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewChannelID(rng.Uint64())
			postDecode := ChannelID{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(t, err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(t, err)

			assert.True(t, preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *ChannelID", func(t *testing.T) {
		var np *ChannelID

		out, err := cbor.DumpObject(np)
		assert.NoError(t, err)

		out2, err := cbor.DumpObject(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestChannelIDJsonMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("JSON unmarshal(marshal(ChannelID)) == identity(ChannelID)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewChannelID(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(t, err)

			var unmarshaled ChannelID
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(t, err)

			assert.True(t, toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *ChannelID", func(t *testing.T) {
		var np *ChannelID

		out, err := json.Marshal(np)
		assert.NoError(t, err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}
