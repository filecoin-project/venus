package types

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func BigIntFromString(s string) big.Int {
	bigInt, _ := new(big.Int).SetString(s, 10)
	return *bigInt
}

func TestFILToAttoFIL(t *testing.T) {
	tf.UnitTest(t)

	x := NewAttoFILFromFIL(2)
	v := big.NewInt(10)
	v = v.Exp(v, big.NewInt(18), nil)
	v = v.Mul(v, big.NewInt(2))
	assert.True(t, NewAttoFIL(v).Equal(x))
}

func TestAttoFILCreation(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(123)
	assert.IsType(t, AttoFIL{}, a)

	ab := a.Bytes()
	b := NewAttoFILFromBytes(ab)
	assert.Equal(t, a, b)

	as := a.String()
	assert.Equal(t, as, "123")
	c, ok := NewAttoFILFromFILString(as)
	assert.True(t, ok)
	assert.Equal(t, a, c)
	d, ok := NewAttoFILFromString("123000000000000000000", 10)
	assert.True(t, ok)
	assert.Equal(t, a, d)

	_, ok = NewAttoFILFromFILString("asdf")
	assert.False(t, ok)
}

func TestZeroAttoFIL(t *testing.T) {
	tf.UnitTest(t)

	z := NewAttoFILFromFIL(0)

	assert.True(t, z.Equal(AttoFIL{}))
	assert.True(t, ZeroAttoFIL.Equal(AttoFIL{}))
}

func TestAttoFILComparison(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(123)
	b := NewAttoFILFromFIL(123)
	c := NewAttoFILFromFIL(456)

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

	t.Run("treats zero values as zero", func(t *testing.T) {
		d := ZeroAttoFIL.Sub(a)
		var zeroValue AttoFIL

		assert.True(t, zeroValue.Equal(ZeroAttoFIL))
		assert.True(t, ZeroAttoFIL.Equal(zeroValue))
		assert.True(t, d.LessThan(zeroValue))
		assert.True(t, zeroValue.GreaterThan(d))
		assert.True(t, c.GreaterThan(zeroValue))
		assert.True(t, zeroValue.LessThan(c))
	})
}

func TestAttoFILAddition(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(123)
	b := NewAttoFILFromFIL(456)

	t.Run("handles addition", func(t *testing.T) {
		aStr := a.String()
		bStr := b.String()
		sum := a.Add(b)

		assert.Equal(t, NewAttoFILFromFIL(579), sum)

		// Storage is not reused
		assert.NotEqual(t, &a, &sum)
		assert.NotEqual(t, &b, &sum)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats zero values as zero", func(t *testing.T) {
		var x, z AttoFIL

		assert.True(t, z.Add(a).Equal(a))
		assert.True(t, a.Add(z).Equal(a))
		assert.True(t, a.Add(AttoFIL{}).Equal(a))
		assert.True(t, z.Add(x).Equal(AttoFIL{}))
		assert.True(t, z.Add(AttoFIL{}).Equal(x))
	})
}

func TestAttoFILSubtraction(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(456)
	b := NewAttoFILFromFIL(123)

	t.Run("handles subtraction", func(t *testing.T) {
		aStr := a.String()
		bStr := b.String()
		delta := a.Sub(b)

		assert.Equal(t, delta, NewAttoFILFromFIL(333))

		// Storage is not reused
		assert.NotEqual(t, &a, &delta)
		assert.NotEqual(t, &b, &delta)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats zero values as zero", func(t *testing.T) {
		var z AttoFIL

		assert.True(t, a.Sub(z).Equal(a))
		assert.True(t, a.Sub(AttoFIL{}).Equal(a))
		assert.True(t, z.Sub(z).Equal(z))
		assert.True(t, z.Sub(AttoFIL{}).Equal(AttoFIL{}))
	})
}

func TestMulInt(t *testing.T) {
	tf.UnitTest(t)

	multiplier := big.NewInt(25)
	attoFIL := NewAttoFIL(big.NewInt(1000))

	t.Run("correctly multiplies the values and returns an AttoFIL", func(t *testing.T) {
		expected := NewAttoFIL(big.NewInt(25000))
		assert.Equal(t, attoFIL.MulBigInt(multiplier), expected)
	})
}

func TestDivCeil(t *testing.T) {
	tf.UnitTest(t)

	x := NewAttoFIL(big.NewInt(200))

	t.Run("returns exactly the dividend when y divides x", func(t *testing.T) {
		actual := x.DivCeil(NewAttoFIL(big.NewInt(10)))
		assert.Equal(t, NewAttoFIL(big.NewInt(20)), actual)
	})

	t.Run("rounds up when y does not divide x", func(t *testing.T) {
		actual := x.DivCeil(NewAttoFIL(big.NewInt(9)))
		assert.Equal(t, NewAttoFIL(big.NewInt(23)), actual)
	})
}

func TestPriceCalculation(t *testing.T) {
	tf.UnitTest(t)

	price := NewAttoFILFromFIL(123)
	numBytes := NewBytesAmount(10)

	t.Run("calculates prices by multiplying with BytesAmount", func(t *testing.T) {
		priceStr := price.String()
		numBytesStr := numBytes.String()

		total := price.CalculatePrice(numBytes)
		assert.Equal(t, total, NewAttoFILFromFIL(1230))

		// Storage is not reused
		assert.NotEqual(t, &price, &total)
		assert.NotEqual(t, &numBytes, &total)

		// Values have not changed.
		assert.Equal(t, priceStr, price.String())
		assert.Equal(t, numBytesStr, numBytes.String())
	})

	t.Run("treats zero values as zero", func(t *testing.T) {
		var nt AttoFIL
		var nb *BytesAmount

		assert.True(t, price.CalculatePrice(nil).Equal(ZeroAttoFIL))
		assert.True(t, nt.CalculatePrice(numBytes).Equal(ZeroAttoFIL))
		assert.True(t, price.CalculatePrice(nb).Equal(ZeroAttoFIL))
		assert.True(t, nt.CalculatePrice(nb).Equal(ZeroAttoFIL))
	})
}

func TestAttoFILCborMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("CBOR decode(encode(AttoFIL)) == identity(AttoFIL)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			preEncode := NewAttoFILFromFIL(rng.Uint64())
			postDecode := AttoFIL{}

			out, err := encoding.Encode(preEncode)
			assert.NoError(t, err)

			err = encoding.Decode(out, &postDecode)
			assert.NoError(t, err)

			assert.True(t, preEncode.Equal(postDecode), "pre: %s post: %s", preEncode.String(), postDecode.String())
		}
	})
	t.Run("cannot CBOR encode nil as *AttoFIL", func(t *testing.T) {
		var np *AttoFIL

		out, err := encoding.Encode(np)
		assert.NoError(t, err)

		out2, err := encoding.Encode(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestAttoFILJsonMarshaling(t *testing.T) {
	tf.UnitTest(t)

	t.Run("JSON unmarshal(marshal(AttoFIL)) == identity(AttoFIL)", func(t *testing.T) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		for i := 0; i < 100; i++ {
			toBeMarshaled := NewAttoFILFromFIL(rng.Uint64())

			marshaled, err := json.Marshal(toBeMarshaled)
			assert.NoError(t, err)

			var unmarshaled AttoFIL
			err = json.Unmarshal(marshaled, &unmarshaled)
			assert.NoError(t, err)

			assert.True(t, toBeMarshaled.Equal(unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
		}
	})

	t.Run("unmarshal(marshal(AttoFIL)) == AttoFIL for decimal FIL", func(t *testing.T) {
		toBeMarshaled, _ := NewAttoFILFromFILString("912129289198393.123456789012345678")

		marshaled, err := json.Marshal(toBeMarshaled)
		assert.NoError(t, err)

		var unmarshaled AttoFIL
		err = json.Unmarshal(marshaled, &unmarshaled)
		assert.NoError(t, err)

		assert.True(t, toBeMarshaled.Equal(unmarshaled), "should be equal - toBeMarshaled: %s unmarshaled: %s)", toBeMarshaled.String(), unmarshaled.String())
	})

	t.Run("cannot JSON marshall nil as *AttoFIL", func(t *testing.T) {
		var np *AttoFIL

		out, err := json.Marshal(np)
		assert.NoError(t, err)

		out2, err := json.Marshal(ZeroAttoFIL)
		assert.NoError(t, err)

		assert.NotEqual(t, out, out2)
	})
}

func TestAttoFILIsPositive(t *testing.T) {
	tf.UnitTest(t)

	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var zeroValue AttoFIL

	t.Run("returns false if zero", func(t *testing.T) {
		assert.False(t, z.IsPositive())
		assert.False(t, zeroValue.IsPositive())
	})

	t.Run("returns true if greater than zero", func(t *testing.T) {
		assert.True(t, p.IsPositive())
	})

	t.Run("returns false if less than zero", func(t *testing.T) {
		assert.False(t, n.IsPositive(), "IsPositive(%s)", n.String())
	})
}

func TestAttoFILIsNegative(t *testing.T) {
	tf.UnitTest(t)

	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var zeroValue AttoFIL

	t.Run("returns false if zero", func(t *testing.T) {
		assert.False(t, z.IsNegative())
		assert.False(t, zeroValue.IsNegative())
	})

	t.Run("returns false if greater than zero", func(t *testing.T) {
		assert.False(t, p.IsNegative())
	})

	t.Run("returns true if less than zero", func(t *testing.T) {
		assert.True(t, n.IsNegative(), "IsNegative(%s)", n.String())
	})
}

func TestAttoFILIsZero(t *testing.T) {
	tf.UnitTest(t)

	p := NewAttoFILFromFIL(100)      // positive
	z := NewAttoFILFromFIL(0)        // zero
	n := NewAttoFILFromFIL(0).Sub(p) // negative
	var zeroValue AttoFIL

	t.Run("returns true if zero token", func(t *testing.T) {
		assert.True(t, z.IsZero())
		assert.True(t, zeroValue.IsZero())
	})

	t.Run("returns false if greater than zero token", func(t *testing.T) {
		assert.False(t, p.IsZero())
	})

	t.Run("returns false if less than zero token", func(t *testing.T) {
		assert.False(t, n.IsZero())
	})
}

func TestString(t *testing.T) {
	tf.UnitTest(t)

	// A very large number of attoFIL
	attoFIL, _ := new(big.Int).SetString("912129289198393123456789012345678", 10)
	assert.Equal(t, "912129289198393.123456789012345678", NewAttoFIL(attoFIL).String())

	// A multiple of 1000 attoFIL
	attoFIL, _ = new(big.Int).SetString("9123372036854775000", 10)
	assert.Equal(t, "9.123372036854775", NewAttoFIL(attoFIL).String())

	// Less than 10^18 attoFIL
	attoFIL, _ = new(big.Int).SetString("36854775878", 10)
	assert.Equal(t, "0.000000036854775878", NewAttoFIL(attoFIL).String())

	// A multiple of 100 attFIL that is less than 10^18
	attoFIL, _ = new(big.Int).SetString("36854775800", 10)
	assert.Equal(t, "0.0000000368547758", NewAttoFIL(attoFIL).String())

	// A number of attFIL that is an integer number of FIL
	attoFIL, _ = new(big.Int).SetString("123000000000000000000", 10)
	assert.Equal(t, "123", NewAttoFIL(attoFIL).String())
}

func TestNewAttoFILFromFILString(t *testing.T) {
	tf.UnitTest(t)

	t.Run("parses legitimate values correctly", func(t *testing.T) {
		attoFIL, _ := NewAttoFILFromFILString(".12345")
		assert.Equal(t, BigIntFromString("123450000000000000"), attoFIL.val)

		attoFIL, _ = NewAttoFILFromFILString("000000.000000")
		assert.Equal(t, BigIntFromString("0"), attoFIL.val)

		attoFIL, _ = NewAttoFILFromFILString("0000.12345")
		assert.Equal(t, BigIntFromString("123450000000000000"), attoFIL.val)

		attoFIL, _ = NewAttoFILFromFILString("12345.0")
		assert.Equal(t, BigIntFromString("12345000000000000000000"), attoFIL.val)

		attoFIL, _ = NewAttoFILFromFILString("12345")
		assert.Equal(t, BigIntFromString("12345000000000000000000"), attoFIL.val)
	})

	t.Run("rejects nonsense values", func(t *testing.T) {
		_, ok := NewAttoFILFromFILString("notanumber")
		assert.False(t, ok)

		_, ok = NewAttoFILFromFILString("384042.wat")
		assert.False(t, ok)

		_, ok = NewAttoFILFromFILString("78wat")
		assert.False(t, ok)

		_, ok = NewAttoFILFromFILString("1234567890abcde")
		assert.False(t, ok)

		_, ok = NewAttoFILFromFILString("127.0.0.1")
		assert.False(t, ok)
	})
}
