package types

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmPbqRavwDZLfmpeW6eoyAoQ5rT2LoCW98JhvRc22CqkZS/go-ipld-cbor"

	"github.com/stretchr/testify/assert"
)

func TestFILToAttoFIL(t *testing.T) {
	assert := assert.New(t)

	x := NewAttoFILFromFIL(2)
	v := big.NewInt(10)
	v = v.Exp(v, big.NewInt(18), nil)
	v = v.Mul(v, big.NewInt(2))
	assert.True(NewAttoFIL(v).Equal(x))
}

func TestAttoFILCreation(t *testing.T) {
	assert := assert.New(t)

	a := NewAttoFILFromFIL(123)
	assert.IsType(&AttoFIL{}, a)

	ab := a.Bytes()
	b := NewAttoFILFromBytes(ab)
	assert.Equal(a, b)

	as := a.String()
	assert.Equal(as, "123")
	c, ok := NewAttoFILFromFILString(as, 10)
	assert.True(ok)
	assert.Equal(a, c)
	d, ok := NewAttoFILFromString("123000000000000000000", 10)
	assert.True(ok)
	assert.Equal(a, d)

	_, ok = NewAttoFILFromFILString("asdf", 10)
	assert.False(ok)
}

func TestZeroAttoFIL(t *testing.T) {
	assert := assert.New(t)

	z := NewAttoFILFromFIL(0)

	assert.Equal(z, ZeroAttoFIL)
	assert.True(z.Equal(nil))
	assert.True(ZeroAttoFIL.Equal(nil))
}

func TestAttoFILComparison(t *testing.T) {
	a := NewAttoFILFromFIL(123)
	b := NewAttoFILFromFIL(123)
	c := NewAttoFILFromFIL(456)

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
		d := ZeroAttoFIL.Sub(a)
		var np *AttoFIL

		assert.True(np.Equal(ZeroAttoFIL))
		assert.True(ZeroAttoFIL.Equal(np))
		assert.True(d.LessThan(np))
		assert.True(np.GreaterThan(d))
		assert.True(c.GreaterThan(np))
		assert.True(np.LessThan(c))
	})
}

func TestAttoFILAddition(t *testing.T) {
	a := NewAttoFILFromFIL(123)
	b := NewAttoFILFromFIL(456)

	t.Run("handles addition", func(t *testing.T) {
		assert := assert.New(t)

		aStr := a.String()
		bStr := b.String()
		sum := a.Add(b)

		assert.Equal(NewAttoFILFromFIL(579), sum)

		// Storage is not reused
		assert.NotEqual(&a, &sum)
		assert.NotEqual(&b, &sum)

		// Values have not changed.
		assert.Equal(aStr, a.String())
		assert.Equal(bStr, b.String())
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var x, z *AttoFIL

		assert.True(z.Add(a).Equal(a))
		assert.True(a.Add(z).Equal(a))
		assert.True(a.Add(nil).Equal(a))
		assert.True(z.Add(x).Equal(nil))
		assert.True(z.Add(nil).Equal(x))
	})
}

func TestAttoFILSubtraction(t *testing.T) {
	a := NewAttoFILFromFIL(456)
	b := NewAttoFILFromFIL(123)

	t.Run("handles subtraction", func(t *testing.T) {
		assert := assert.New(t)

		aStr := a.String()
		bStr := b.String()
		delta := a.Sub(b)

		assert.Equal(delta, NewAttoFILFromFIL(333))

		// Storage is not reused
		assert.NotEqual(&a, &delta)
		assert.NotEqual(&b, &delta)

		// Values have not changed.
		assert.Equal(aStr, a.String())
		assert.Equal(bStr, b.String())
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var z *AttoFIL

		assert.True(a.Sub(z).Equal(a))
		assert.True(a.Sub(nil).Equal(a))
		assert.True(z.Sub(z).Equal(z))
		assert.True(z.Sub(nil).Equal(nil))
	})
}

func TestPriceCalculation(t *testing.T) {
	price := NewAttoFILFromFIL(123)
	numBytes := NewBytesAmount(10)

	t.Run("calculates prices by multiplying with BytesAmount", func(t *testing.T) {
		assert := assert.New(t)
		priceStr := price.String()
		numBytesStr := numBytes.String()

		total := price.CalculatePrice(numBytes)
		assert.Equal(total, NewAttoFILFromFIL(1230))

		// Storage is not reused
		assert.NotEqual(&price, &total)
		assert.NotEqual(&numBytes, &total)

		// Values have not changed.
		assert.Equal(priceStr, price.String())
		assert.Equal(numBytesStr, numBytes.String())
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var nt *AttoFIL
		var nb *BytesAmount

		assert.Equal(price.CalculatePrice(nil), ZeroAttoFIL)
		assert.Equal(nt.CalculatePrice(numBytes), ZeroAttoFIL)
		assert.Equal(price.CalculatePrice(nb), ZeroAttoFIL)
		assert.Equal(nt.CalculatePrice(nb), ZeroAttoFIL)
	})
}

func TestAttoFILCborMarshaling(t *testing.T) {
	t.Run("CBOR decode(encode(AttoFIL)) == identity(AttoFIL)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewAttoFILFromFIL(rng.Uint64())
			postDecode := AttoFIL{}

			out, err := cbor.DumpObject(preEncode)
			assert.NoError(err)

			err = cbor.DecodeInto(out, &postDecode)
			assert.NoError(err)

			assert.True(preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *AttoFIL", func(t *testing.T) {
		assert := assert.New(t)

		var np *AttoFIL

		out, err := cbor.DumpObject(np)
		assert.NoError(err)

		out2, err := cbor.DumpObject(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestAttoFILJsonMarshaling(t *testing.T) {
	t.Run("JSON unmarshal(marshal(AttoFIL)) == identity(AttoFIL)", func(t *testing.T) {
		assert := assert.New(t)

		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewAttoFILFromFIL(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(err)

			var unmarshaled AttoFIL
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(err)

			assert.True(toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *AttoFIL", func(t *testing.T) {
		assert := assert.New(t)

		var np *AttoFIL

		out, err := json.Marshal(np)
		assert.NoError(err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})
}

func TestAttoFILIsPositive(t *testing.T) {
	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var np *AttoFIL

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

func TestAttoFILIsNegative(t *testing.T) {
	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var np *AttoFIL

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

func TestAttoFILIsZero(t *testing.T) {
	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var np *AttoFIL

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
