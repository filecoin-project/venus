package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestChannelIDCreation(t *testing.T) {
	assert := assert.New(t)

	a := NewChannelID(123)
	assert.IsType(&ChannelID{}, a)

	ab := a.Bytes()
	b := NewChannelIDFromBytes(ab)
	assert.Equal(a, b)

	as := a.String()
	assert.Equal(as, "123")
	c, ok := NewChannelIDFromString(as, 10)
	assert.True(ok)
	assert.Equal(a, c)

	next := a.Inc()
	assert.Equal("124", next.String())

	_, ok = NewChannelIDFromString("asdf", 10)
	assert.False(ok)
}

func TestChannelIDCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(ChannelID)) == identity(ChannelID)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewChannelID(rng.Uint64())
			postDecode := ChannelID{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(err)

			assert.True(preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *ChannelID", func(t *testing.T) {
		assert := assert.New(t)

		var np *ChannelID

		out, err := cbor.DumpObject(np)
		assert.NoError(err)

		out2, err := cbor.DumpObject(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestChannelIDJsonMarshaling(t *testing.T) {
	t.Run("JSON unmarshal(marshal(ChannelID)) == identity(ChannelID)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewChannelID(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(err)

			var unmarshaled ChannelID
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(err)

			assert.True(toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *ChannelID", func(t *testing.T) {
		assert := assert.New(t)

		var np *ChannelID

		out, err := json.Marshal(np)
		assert.NoError(err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}
