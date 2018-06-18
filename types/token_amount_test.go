package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"

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
	assert.True(z.Equal(nil))
	assert.True(ZeroToken.Equal(nil))
}

func TestTokenAmountComparison(t *testing.T) {
	a := NewTokenAmount(123)
	b := NewTokenAmount(123)
	c := NewTokenAmount(456)

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

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		d := ZeroToken.Sub(a)
		var np *TokenAmount

		assert.True(np.Equal(ZeroToken))
		assert.True(ZeroToken.Equal(np))
		assert.True(d.LessThan(np))
		assert.True(np.GreaterThan(d))
		assert.True(c.GreaterThan(np))
		assert.True(np.LessThan(c))
	})
}

func TestTokenAmountAddition(t *testing.T) {
	a := NewTokenAmount(123)
	b := NewTokenAmount(456)

	t.Run("handles addition", func(t *testing.T) {
		assert := assert.New(t)

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
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var x, z *TokenAmount

		assert.True(z.Add(a).Equal(a))
		assert.True(a.Add(z).Equal(a))
		assert.True(a.Add(nil).Equal(a))
		assert.True(z.Add(x).Equal(nil))
		assert.True(z.Add(nil).Equal(x))
	})
}

func TestTokenAmountSubtraction(t *testing.T) {
	a := NewTokenAmount(456)
	b := NewTokenAmount(123)

	t.Run("handles subtraction", func(t *testing.T) {
		assert := assert.New(t)

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
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var z *TokenAmount

		assert.True(a.Sub(z).Equal(a))
		assert.True(a.Sub(nil).Equal(a))
		assert.True(z.Sub(z).Equal(z))
		assert.True(z.Sub(nil).Equal(nil))
	})
}

func TestTokenAmountPriceCalculation(t *testing.T) {
	price := NewTokenAmount(123)
	numBytes := NewBytesAmount(10)

	t.Run("calculates prices by multiplying with BytesAmount", func(t *testing.T) {
		assert := assert.New(t)
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
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var nt *TokenAmount
		var nb *BytesAmount

		assert.Equal(price.CalculatePrice(nil), ZeroToken)
		assert.Equal(nt.CalculatePrice(numBytes), ZeroToken)
		assert.Equal(price.CalculatePrice(nb), ZeroToken)
		assert.Equal(nt.CalculatePrice(nb), ZeroToken)
	})
}

func TestTokenAmountCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(TokenAmount)) == identity(TokenAmount)", func(t *testing.T) {
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
	t.Run("cannot CBOR encode nil as *TokenAmount", func(t *testing.T) {
		assert := assert.New(t)

		var np *TokenAmount

		out, err := cbor.DumpObject(np)
		assert.NoError(err)

		out2, err := cbor.DumpObject(ZeroToken)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestTokenAmountJsonMarshaling(t *testing.T) {
	t.Run("JSON unmarshal(marshal(TokenAmount)) == identity(TokenAmount)", func(t *testing.T) {
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
	t.Run("cannot JSON marshall nil as *TokenAmount", func(t *testing.T) {
		assert := assert.New(t)

		var np *TokenAmount

		out, err := json.Marshal(np)
		assert.NoError(err)

		out2, err := json.Marshal(ZeroToken)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestTokenAmountIsPositive(t *testing.T) {
	p := NewTokenAmount(100)      // positive
	z := NewTokenAmount(0)        // zero
	n := NewTokenAmount(0).Sub(p) // negative
	var np *TokenAmount

	t.Run("returns false if zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(z.IsPositive())
		assert.False(np.IsPositive())
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
	var np *TokenAmount

	t.Run("returns false if zero", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(z.IsNegative())
		assert.False(np.IsNegative())
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
	var np *TokenAmount

	t.Run("returns true if zero token", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(z.IsZero())
		assert.True(np.IsZero())
	})

	t.Run("returns false if greater than zero token", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(p.IsZero())
	})

	t.Run("returns false if less than zero token", func(t *testing.T) {
		assert := assert.New(t)
		assert.False(n.IsZero())
	})
}
