package types

import (
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func TestBytesAmountCreation(t *testing.T) {
	tf.UnitTest(t)

	a := NewBytesAmount(123)
	assert.IsType(t, &BytesAmount{}, a)

	ab := a.Bytes()
	b := NewBytesAmountFromBytes(ab)
	assert.Equal(t, a, b)

	as := a.String()
	assert.Equal(t, as, "123")
	c, ok := NewBytesAmountFromString(as, 10)
	assert.True(t, ok)
	assert.Equal(t, a, c)

	_, ok = NewBytesAmountFromString("asdf", 10)
	assert.False(t, ok)
}

func TestZeroBytes(t *testing.T) {
	tf.UnitTest(t)

	z := NewBytesAmount(0)

	assert.Equal(t, z, ZeroBytes)
	assert.True(t, z.Equal(nil))
	assert.True(t, ZeroBytes.Equal(nil))

}

func TestBytesAmountComparison(t *testing.T) {
	tf.UnitTest(t)

	a := NewBytesAmount(123)
	b := NewBytesAmount(123)
	c := NewBytesAmount(456)

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

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		d := ZeroBytes.Sub(a)
		var np *BytesAmount

		assert.True(t, np.Equal(ZeroBytes))
		assert.True(t, ZeroBytes.Equal(np))
		assert.True(t, d.LessThan(np))
		assert.True(t, np.GreaterThan(d))
		assert.True(t, c.GreaterThan(np))
		assert.True(t, np.LessThan(c))
	})
}

func TestBytesAmountAddition(t *testing.T) {
	tf.UnitTest(t)

	a := NewBytesAmount(123)
	b := NewBytesAmount(456)

	t.Run("handles addition", func(t *testing.T) {
		aStr := a.String()
		bStr := b.String()
		sum := a.Add(b)

		assert.Equal(t, sum, NewBytesAmount(579))

		// Storage is not reused
		assert.NotEqual(t, &a, &sum)
		assert.NotEqual(t, &b, &sum)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		var x, z *BytesAmount

		assert.True(t, z.Add(a).Equal(a))
		assert.True(t, a.Add(z).Equal(a))
		assert.True(t, a.Add(nil).Equal(a))
		assert.True(t, z.Add(x).Equal(nil))
		assert.True(t, z.Add(nil).Equal(x))
	})

}

func TestBytesAmountSubtraction(t *testing.T) {
	tf.UnitTest(t)

	a := NewBytesAmount(456)
	b := NewBytesAmount(123)

	t.Run("handles subtraction", func(t *testing.T) {
		aStr := a.String()
		bStr := b.String()
		delta := a.Sub(b)

		assert.Equal(t, delta, NewBytesAmount(333))

		// Storage is not reused
		assert.NotEqual(t, &a, &delta)
		assert.NotEqual(t, &b, &delta)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats nil pointers as zero", func(t *testing.T) {
		var z *BytesAmount

		assert.True(t, a.Sub(z).Equal(a))
		assert.True(t, a.Sub(nil).Equal(a))
		assert.True(t, z.Sub(z).Equal(z))
		assert.True(t, z.Sub(nil).Equal(nil))
	})

}

func TestBytesAmountNegative(t *testing.T) {
	tf.UnitTest(t)

	a, ok := NewBytesAmountFromString("-300", 10)
	assert.True(t, ok)

	assert.Equal(t, NewBytesAmount(100), a.Add(NewBytesAmount(400)))
}

func TestBytesAmountCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(BytesAmount)) == identity(BytesAmount)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewBytesAmount(rng.Uint64())
			postDecode := BytesAmount{}

			out, err := encoding.Encode(preEncode)
			assert.NoError(t, err)

			err = encoding.Decode(out, &postDecode)
			assert.NoError(t, err)

			assert.True(t, preEncode.Equal(&postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})

	t.Run("cannot CBOR encode nil as *BytesAmount", func(t *testing.T) {
		var np *BytesAmount

		out, err := encoding.Encode(np)
		assert.NoError(t, err)

		out2, err := encoding.Encode(ZeroBytes)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestBytesAmountJsonMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("JSON unmarshal(marshal(BytesAmount)) == identity(BytesAmount)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewBytesAmount(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(t, err)

			var unmarshaled BytesAmount
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(t, err)

			assert.True(t, toBeMarshaled.Equal(&unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})
	t.Run("cannot JSON marshall nil as *BytesAmount", func(t *testing.T) {
		var np *BytesAmount

		out, err := json.Marshal(np)
		assert.NoError(t, err)

		out2, err := json.Marshal(ZeroBytes)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})

}

func TestBytesAmountIsPositive(t *testing.T) {
	tf.UnitTest(t)

	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

	t.Run("returns false if zero", func(t *testing.T) {
		assert.False(t, z.IsPositive())
		assert.False(t, np.IsPositive())
	})

	t.Run("returns true if greater than zero", func(t *testing.T) {
		assert.True(t, p.IsPositive())
	})

	t.Run("returns false if less than zero", func(t *testing.T) {
		assert.False(t, n.IsPositive(), "IsPositive(%s)", n.String())
	})
}

func TestBytesAmountIsNegative(t *testing.T) {
	tf.UnitTest(t)

	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

	t.Run("returns false if zero", func(t *testing.T) {
		assert.False(t, z.IsNegative())
		assert.False(t, np.IsNegative())
	})

	t.Run("returns false if greater than zero", func(t *testing.T) {
		assert.False(t, p.IsNegative())
	})

	t.Run("returns true if less than zero", func(t *testing.T) {
		assert.True(t, n.IsNegative(), "IsNegative(%s)", n.String())
	})
}

func TestBytesAmountIsZero(t *testing.T) {
	tf.UnitTest(t)

	p := NewBytesAmount(100)      // positive
	z := NewBytesAmount(0)        // zero
	n := NewBytesAmount(0).Sub(p) // negative
	var np *BytesAmount

	t.Run("returns true if zero bytes", func(t *testing.T) {
		assert.True(t, z.IsZero())
		assert.True(t, np.IsZero())
	})

	t.Run("returns false if greater than zero bytes", func(t *testing.T) {
		assert.False(t, p.IsZero())
	})

	t.Run("returns false if less than zero bytes", func(t *testing.T) {
		assert.False(t, n.IsZero())
	})
}

func TestBytesAmountMul(t *testing.T) {
	tf.UnitTest(t)

	a := NewBytesAmount(8)
	b := NewBytesAmount(9)

	aStr := a.String()
	bStr := b.String()
	mul := a.Mul(b)

	assert.Equal(t, mul, NewBytesAmount(8*9))

	// Storage is not reused
	assert.NotEqual(t, &a, &mul)
	assert.NotEqual(t, &b, &mul)

	// Values have not changed.
	assert.Equal(t, aStr, a.String())
	assert.Equal(t, bStr, b.String())
}
