package types

import (
	"testing"

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
