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

func TestBlockHeightCreation(t *testing.T) {
	tf.UnitTest(t)

	a := NewBlockHeight(123)
	assert.IsType(t, &BlockHeight{}, a)

	ab := a.Bytes()
	b := NewBlockHeightFromBytes(ab)
	assert.Equal(t, a, b)

	as := a.String()
	assert.Equal(t, as, "123")
	c, ok := NewBlockHeightFromString(as, 10)
	assert.True(t, ok)
	assert.Equal(t, a, c)

	_, ok = NewBlockHeightFromString("asdf", 10)
	assert.False(t, ok)
}

func TestBlockHeightCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(BlockHeight)) == identity(BlockHeight)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewBlockHeight(rng.Uint64())
			postDecode := BlockHeight{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(t, err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(t, err)

			assert.True(t, preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *BlockHeight", func(t *testing.T) {
		var np *BlockHeight

		out, err := cbor.DumpObject(np)
		assert.NoError(t, err)

		out2, err := cbor.DumpObject(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestBlockHeightJsonMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("JSON unmarshal(marshal(BlockHeight)) == identity(BlockHeight)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewBlockHeight(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(t, err)

			var unmarshaled BlockHeight
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(t, err)

			assert.True(t, toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *BlockHeight", func(t *testing.T) {
		var np *BlockHeight

		out, err := json.Marshal(np)
		assert.NoError(t, err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestBlockHeightComparison(t *testing.T) {
	tf.UnitTest(t)

	a := NewBlockHeight(123)
	b := NewBlockHeight(123)
	c := NewBlockHeight(456)

	t.Run("handles comparison", func(t *testing.T) {
		assert.True(t, a.Equal(b))
		assert.True(t, b.Equal(a))

		assert.False(t, a.Equal(c))
		assert.False(t, c.Equal(a))

		assert.True(t, a.LessThan(c))
		assert.True(t, a.LessEqual(c))
		assert.True(t, c.GreaterThan(a))
		assert.True(t, c.GreaterEqual(a))
		assert.True(t, a.GreaterEqual(b))
		assert.True(t, a.LessEqual(b))
	})
}
