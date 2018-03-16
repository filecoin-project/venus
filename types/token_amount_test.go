package types

import (
	"testing"

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
