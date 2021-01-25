package types

import (
	specsbig "github.com/filecoin-project/go-state-types/big"
	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
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
	assert.True(t, NewAttoFIL(v).Equals(x))
}

func TestZeroAttoFIL(t *testing.T) {
	tf.UnitTest(t)

	z := NewAttoFILFromFIL(0)
	assert.True(t, ZeroFIL.Equals(z))
}

func TestAttoFILComparison(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(123)
	b := NewAttoFILFromFIL(123)
	c := NewAttoFILFromFIL(456)

	t.Run("handles comparison", func(t *testing.T) {
		assert.True(t, a.Equals(b))
		assert.True(t, b.Equals(a))

		assert.False(t, a.Equals(c))
		assert.False(t, c.Equals(a))

		assert.True(t, a.LessThan(c))
		assert.True(t, a.LessThanEqual(c))
		assert.True(t, c.GreaterThan(a))
		assert.True(t, c.GreaterThanEqual(a))
		assert.True(t, a.GreaterThanEqual(b))
		assert.True(t, a.LessThanEqual(b))
	})

	t.Run("treats ZeroFIL as zero", func(t *testing.T) {
		d := specsbig.Sub(ZeroFIL, a)
		zeroValue := NewAttoFILFromFIL(0)

		assert.True(t, zeroValue.Equals(ZeroFIL))
		assert.True(t, ZeroFIL.Equals(zeroValue))
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
		sum := specsbig.Add(a, b)

		assert.Equal(t, NewAttoFILFromFIL(579), sum)

		// Storage is not reused
		assert.NotEqual(t, &a, &sum)
		assert.NotEqual(t, &b, &sum)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats ZeroFIL as zero", func(t *testing.T) {
		assert.True(t, specsbig.Add(ZeroFIL, a).Equals(a))
		assert.True(t, specsbig.Add(a, ZeroFIL).Equals(a))
	})
}

func TestAttoFILSubtraction(t *testing.T) {
	tf.UnitTest(t)

	a := NewAttoFILFromFIL(456)
	b := NewAttoFILFromFIL(123)

	t.Run("handles subtraction", func(t *testing.T) {
		aStr := a.String()
		bStr := b.String()
		delta := specsbig.Sub(a, b)

		assert.Equal(t, delta, NewAttoFILFromFIL(333))

		// Storage is not reused
		assert.NotEqual(t, &a, &delta)
		assert.NotEqual(t, &b, &delta)

		// Values have not changed.
		assert.Equal(t, aStr, a.String())
		assert.Equal(t, bStr, b.String())
	})

	t.Run("treats ZeroFIL as zero", func(t *testing.T) {
		assert.True(t, specsbig.Sub(a, ZeroFIL).Equals(a))
		assert.True(t, specsbig.Sub(ZeroFIL, ZeroFIL).Equals(ZeroFIL))
	})
}

func TestAttoFILIsZero(t *testing.T) {
	tf.UnitTest(t)

	p := NewAttoFILFromFIL(100)                // positive
	z := NewAttoFILFromFIL(0)                  // zero
	n := specsbig.Sub(NewAttoFILFromFIL(0), p) // negative

	t.Run("returns true if zero token", func(t *testing.T) {
		assert.True(t, z.IsZero())
		assert.True(t, ZeroFIL.IsZero())
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
	assert.Equal(t, "912129289198393123456789012345678", NewAttoFIL(attoFIL).String())

	// A multiple of 1000 attoFIL
	attoFIL, _ = new(big.Int).SetString("9123372036854775000", 10)
	assert.Equal(t, "9123372036854775000", NewAttoFIL(attoFIL).String())

	// Less than 10^18 attoFIL
	attoFIL, _ = new(big.Int).SetString("36854775878", 10)
	assert.Equal(t, "36854775878", NewAttoFIL(attoFIL).String())

	// A multiple of 100 attFIL that is less than 10^18
	attoFIL, _ = new(big.Int).SetString("36854775800", 10)
	assert.Equal(t, "36854775800", NewAttoFIL(attoFIL).String())

	// A number of attFIL that is an integer number of FIL
	attoFIL, _ = new(big.Int).SetString("123000000000000000000", 10)
	assert.Equal(t, "123000000000000000000", NewAttoFIL(attoFIL).String())
}

func TestNewAttoFILFromFILString(t *testing.T) {
	tf.UnitTest(t)

	t.Run("parses legitimate values correctly", func(t *testing.T) {
		attoFIL, _ := NewAttoFILFromFILString(".12345")
		assert.Equal(t, BigIntFromString("123450000000000000"), *attoFIL.Int)

		attoFIL, _ = NewAttoFILFromFILString("000000.000000")
		assert.Equal(t, BigIntFromString("0"), *attoFIL.Int)

		attoFIL, _ = NewAttoFILFromFILString("0000.12345")
		assert.Equal(t, BigIntFromString("123450000000000000"), *attoFIL.Int)

		attoFIL, _ = NewAttoFILFromFILString("12345.0")
		assert.Equal(t, BigIntFromString("12345000000000000000000"), *attoFIL.Int)

		attoFIL, _ = NewAttoFILFromFILString("12345")
		assert.Equal(t, BigIntFromString("12345000000000000000000"), *attoFIL.Int)
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
