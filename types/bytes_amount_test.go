package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestBytesAmountCreation(t *testing.T) {
	assert := assert.New(t)

	a := NewBytesAmount(123)
	assert.IsType(&BytesAmount{}, a)

	ab := a.Bytes()
	b := NewBytesAmountFromBytes(ab)
	assert.Equal(a, b)

	as := a.String()
	assert.Equal(as, "123")
	c, ok := NewBytesAmountFromString(as, 10)
	assert.True(ok)
	assert.Equal(a, c)

	_, ok = NewBytesAmountFromString("asdf", 10)
	assert.False(ok)
}

func TestZeroBytes(t *testing.T) {
	assert := assert.New(t)

	z := NewBytesAmount(0)

	assert.Equal(z, ZeroBytes)
}

func TestBytesAmountComparison(t *testing.T) {
	assert := assert.New(t)

	a := NewBytesAmount(123)
	b := NewBytesAmount(123)
	c := NewBytesAmount(456)

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
}

func TestBytesAmountAddition(t *testing.T) {
	assert := assert.New(t)

	a := NewBytesAmount(123)
	b := NewBytesAmount(456)

	aStr := a.String()
	bStr := b.String()
	sum := a.Add(b)

	assert.Equal(sum, NewBytesAmount(579))

	// Storage is not reused
	assert.NotEqual(&a, &sum)
	assert.NotEqual(&b, &sum)

	// Values have not changed.
	assert.Equal(aStr, a.String())
	assert.Equal(bStr, b.String())
}

func TestBytesAmountSubtraction(t *testing.T) {
	assert := assert.New(t)

	a := NewBytesAmount(456)
	b := NewBytesAmount(123)
	aStr := a.String()
	bStr := b.String()
	delta := a.Sub(b)

	assert.Equal(delta, NewBytesAmount(333))

	// Storage is not reused
	assert.NotEqual(&a, &delta)
	assert.NotEqual(&b, &delta)

	// Values have not changed.
	assert.Equal(aStr, a.String())
	assert.Equal(bStr, b.String())
}

func TestBytesAmountCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(BytesAmount)) == identity(BytesAmount)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewBytesAmount(rng.Uint64())
			postDecode := BytesAmount{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(err)

			assert.True(preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
}

func TestBytesAmountJsonMarshaling(t *testing.T) {
	t.Run("JSON unmarshal(marshal(BytesAmount)) == identity(BytesAmount)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewBytesAmount(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(err)

			var unmarshaled BytesAmount
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(err)

			assert.True(toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
}

func TestBytesAmountIsPositive(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative

	t.Run("returns false if zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(z.IsPositive())
	})

	t.Run("returns true if greater than zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(p.IsPositive())
	})

	t.Run("returns false if less than zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(n.IsPositive(), "IsPositive(%s)", n.String())
	})
}

func TestBytesAmountIsNegative(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative

	t.Run("returns false if zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(z.IsNegative())
	})

	t.Run("returns false if greater than zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(p.IsNegative())
	})

	t.Run("returns true if less than zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(n.IsNegative(), "IsNegative(%s)", n.String())
	})
}

func TestBytesAmountIsZero(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative

	t.Run("returns true if zero bytes", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(z.IsZero())
	})

	t.Run("returns false if greater than zero bytes", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(p.IsZero())
	})

	t.Run("returns false if less than zero bytes", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(n.IsZero())
	})
}

func TestBytesAmountSet(t *testing.T) {
	assert := assert.New(t)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < 100; i++ {
		x := NewBytesAmount(rng.Uint64())
		y := NewBytesAmount(0).Set(x)
		assert.True(y.Equal(x), "should be equal - x: %s y: %s)", x.String(), y.String())
	}
}
