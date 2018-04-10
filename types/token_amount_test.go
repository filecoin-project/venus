package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestTokenAmountCreation(t *testing.T) {
	assert := assert.New(t)

	a := NewTokenAmount(123)
	assert.IsType(&TokenAmount{}, a)

	ab := a.Bytes()
	b := NewTokenAmountFromBytes(ab)
	assert.Equal(a, b)

	as := a.String()
	assert.Equal(as, "123")
	c, ok := NewTokenAmountFromString(as, 10)
	assert.True(ok)
	assert.Equal(a, c)

	_, ok = NewTokenAmountFromString("asdf", 10)
	assert.False(ok)
}

func TestZeroToken(t *testing.T) {
	assert := assert.New(t)

	z := NewTokenAmount(0)

	assert.Equal(z, ZeroToken)
}

func TestTokenAmountComparison(t *testing.T) {
	assert := assert.New(t)

	a := NewTokenAmount(123)
	b := NewTokenAmount(123)
	c := NewTokenAmount(456)

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

func TestTokenAmountAddition(t *testing.T) {
	assert := assert.New(t)

	a := NewTokenAmount(123)
	b := NewTokenAmount(456)

	aStr := a.String()
	bStr := b.String()
	sum := a.Add(b)

	assert.Equal(sum, NewTokenAmount(579))

	// Storage is not reused
	assert.NotEqual(&a, &sum)
	assert.NotEqual(&b, &sum)

	// Values have not changed.
	assert.Equal(aStr, a.String())
	assert.Equal(bStr, b.String())
}

func TestTokenAmountSubtraction(t *testing.T) {
	assert := assert.New(t)

	a := NewTokenAmount(456)
	b := NewTokenAmount(123)
	aStr := a.String()
	bStr := b.String()
	delta := a.Sub(b)

	assert.Equal(delta, NewTokenAmount(333))

	// Storage is not reused
	assert.NotEqual(&a, &delta)
	assert.NotEqual(&b, &delta)

	// Values have not changed.
	assert.Equal(aStr, a.String())
	assert.Equal(bStr, b.String())
}

func TestTokenAmountPriceCalculation(t *testing.T) {
	assert := assert.New(t)

	price := NewTokenAmount(123)
	numBytes := NewBytesAmount(10)
	priceStr := price.String()
	numBytesStr := numBytes.String()

	total := price.CalculatePrice(numBytes)
	assert.Equal(total, NewTokenAmount(1230))

	// Storage is not reused
	assert.NotEqual(&price, &total)
	assert.NotEqual(&numBytes, &total)

	// Values have not changed.
	assert.Equal(priceStr, price.String())
	assert.Equal(numBytesStr, numBytes.String())
}

func TestTokenAmountCborMarshaling(t *testing.T) {
	t.Run("CBOR encode(decode(TokenAmount)) == identity(TokenAmount)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewTokenAmount(rng.Uint64())
			postDecode := TokenAmount{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(err)

			assert.True(preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
}

func TestTokenAmountJsonMarshaling(t *testing.T) {
	t.Run("JSON mashal(unmarshal(TokenAmount)) == identity(TokenAmount)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewTokenAmount(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(err)

			var unmarshaled TokenAmount
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(err)

			assert.True(toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
}

func TestTokenAmountIsPositive(t *testing.T) {
	p := NewTokenAmount(100)      // positive
	z := NewTokenAmount(0)        // zero
	n := NewTokenAmount(0).Sub(p) // negative

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

func TestTokenAmountIsNegative(t *testing.T) {
	p := NewTokenAmount(100)      // positive
	z := NewTokenAmount(0)        // zero
	n := NewTokenAmount(0).Sub(p) // negative

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

func TestTokenAmountIsZero(t *testing.T) {
	p := NewTokenAmount(100)      // positive
	z := NewTokenAmount(0)        // zero
	n := NewTokenAmount(0).Sub(p) // negative

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

func TestTokenAmountSet(t *testing.T) {
	assert := assert.New(t)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < 100; i++ {
		x := NewTokenAmount(rng.Uint64())
		y := NewTokenAmount(0).Set(x)
		assert.True(y.Equal(x), "should be equal - x: %s y: %s)", x.String(), y.String())
	}
}
