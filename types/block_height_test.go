package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func TestBlockHeightCreation(t *testing.T) {
	assert := assert.New(t)

	a := NewBlockHeight(123)
	assert.IsType(&BlockHeight{}, a)

	ab := a.Bytes()
	b := NewBlockHeightFromBytes(ab)
	assert.Equal(a, b)

	as := a.String()
	assert.Equal(as, "123")
	c, ok := NewBlockHeightFromString(as, 10)
	assert.True(ok)
	assert.Equal(a, c)

	_, ok = NewBlockHeightFromString("asdf", 10)
	assert.False(ok)
}

func TestBlockHeightCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(BlockHeight)) == identity(BlockHeight)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewBlockHeight(rng.Uint64())
			postDecode := BlockHeight{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(err)

			assert.True(preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *BlockHeight", func(t *testing.T) {
		assert := assert.New(t)

		var np *BlockHeight

		out, err := cbor.DumpObject(np)
		assert.NoError(err)

		out2, err := cbor.DumpObject(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestBlockHeightJsonMarshaling(t *testing.T) {
	t.Run("JSON unmarshal(marshal(BlockHeight)) == identity(BlockHeight)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewBlockHeight(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(err)

			var unmarshaled BlockHeight
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(err)

			assert.True(toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *BlockHeight", func(t *testing.T) {
		assert := assert.New(t)

		var np *BlockHeight

		out, err := json.Marshal(np)
		assert.NoError(err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestBlockHeightComparison(t *testing.T) {
	a := NewBlockHeight(123)
	b := NewBlockHeight(123)
	c := NewBlockHeight(456)

	t.Run("handles comparison", func(t *testing.T) {
		assert := assert.New(t)

		assert.True(a.Equal(b))
		assert.True(b.Equal(a))

		assert.False(a.Equal(c))
		assert.False(c.Equal(a))

		assert.True(a.LessThan(c))
		assert.True(a.LessEqual(c))
		assert.True(c.GreaterThan(a))
		assert.True(c.GreaterEqual(a))
		assert.True(a.GreaterEqual(b))
		assert.True(a.LessEqual(b))
	})
}
