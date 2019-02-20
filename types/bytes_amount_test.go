package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
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
	assert.True(z.Equal(nil))
	assert.True(ZeroBytes.Equal(nil))

}

func TestBytesAmountComparison(t *testing.T) {
	a := NewBytesAmount(123)
	b := NewBytesAmount(123)
	c := NewBytesAmount(456)

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
		d := ZeroBytes.Sub(a)
		var np *BytesAmount

		assert.True(np.Equal(ZeroBytes))
		assert.True(ZeroBytes.Equal(np))
		assert.True(d.LessThan(np))
		assert.True(np.GreaterThan(d))
		assert.True(c.GreaterThan(np))
		assert.True(np.LessThan(c))
	})
}

func TestBytesAmountAddition(t *testing.T) {
	a := NewBytesAmount(123)
	b := NewBytesAmount(456)

	t.Run("handles addition", func(t *testing.T) {
		assert := assert.New(t)

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
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var x, z *BytesAmount

		assert.True(z.Add(a).Equal(a))
		assert.True(a.Add(z).Equal(a))
		assert.True(a.Add(nil).Equal(a))
		assert.True(z.Add(x).Equal(nil))
		assert.True(z.Add(nil).Equal(x))
	})

}

func TestBytesAmountSubtraction(t *testing.T) {
	a := NewBytesAmount(456)
	b := NewBytesAmount(123)

	t.Run("handles subtraction", func(t *testing.T) {
		assert := assert.New(t)

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
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		assert := assert.New(t)
		var z *BytesAmount

		assert.True(a.Sub(z).Equal(a))
		assert.True(a.Sub(nil).Equal(a))
		assert.True(z.Sub(z).Equal(z))
		assert.True(z.Sub(nil).Equal(nil))
	})

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

	t.Run("cannot CBOR encode nil as *BytesAmount", func(t *testing.T) {
		assert := assert.New(t)

		var np *BytesAmount

		out, err := cbor.DumpObject(np)
		assert.NoError(err)

		out2, err := cbor.DumpObject(ZeroBytes)
		assert.NoError(err)

		assert.NotEqual(out, out2)
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
	t.Run("cannot JSON marshall nil as *BytesAmount", func(t *testing.T) {
		assert := assert.New(t)

		var np *BytesAmount

		out, err := json.Marshal(np)
		assert.NoError(err)

		out2, err := json.Marshal(ZeroBytes)
		assert.NoError(err)

		assert.NotEqual(out, out2)
	})

}

func TestBytesAmountIsPositive(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

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

func TestBytesAmountIsNegative(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

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

func TestBytesAmountIsZero(t *testing.T) {
	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

	t.Run("returns true if zero bytes", func(t *testing.T) {
		assert := assert.New(t)
		assert.True(z.IsZero())
		assert.True(np.IsZero())
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

func TestBytesAmountMul(t *testing.T) {
	a := NewBytesAmount(8)
	b := NewBytesAmount(9)

	assert := assert.New(t)

	aStr := a.String()
	bStr := b.String()
	mul := a.Mul(b)

	assert.Equal(mul, NewBytesAmount(8*9))

	// Storage is not reused
	assert.NotEqual(&a, &mul)
	assert.NotEqual(&b, &mul)

	// Values have not changed.
	assert.Equal(aStr, a.String())
	assert.Equal(bStr, b.String())
}
