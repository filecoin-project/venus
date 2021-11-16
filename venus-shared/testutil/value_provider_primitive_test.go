package testutil

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultBytes(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	for i := 0; i < 16; i++ {
		var b []byte
		Provide(t, &b)
		assert.Len(t, b, defaultBytesFixedSize)

		expected := make([]byte, defaultBytesFixedSize)
		local.Read(expected[:])
		assert.Equal(t, expected, b)
	}

}

func TestDefaultString(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	for i := 0; i < 16; i++ {
		var s string
		Provide(t, &s)
		assert.Len(t, s, defaultBytesFixedSize*2)

		expected := make([]byte, defaultBytesFixedSize)
		local.Read(expected[:])
		assert.Equal(t, hex.EncodeToString(expected), s)
	}
}

func TestDefaultInt(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	for i := 0; i < 16; i++ {
		var n int
		Provide(t, &n)
		assert.Equal(t, n, local.Int())
	}
}

func TestDefaultIntSlice(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	var dest []int
	Provide(t, &dest)

	assert.Len(t, dest, 1)
	assert.Equal(t, dest[0], local.Int())
}

func TestDefaultNonNilIntSlice(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	dest := make([]int, 16)
	Provide(t, &dest)

	expected := make([]int, 16)
	for i := range expected {
		expected[i] = local.Int()
	}

	assert.Equal(t, expected, dest)
}

func TestIntSliceWithFixedNumber(t *testing.T) {
	now := int(time.Now().UnixNano())

	dest := make([]int, 16)
	Provide(t, &dest, func(t *testing.T) int {
		return now
	})

	expected := make([]int, 16)
	for i := range expected {
		expected[i] = now
	}

	assert.Equal(t, expected, dest)
}

func TestDefaultIntArray(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	var dest [16]int
	Provide(t, &dest)

	expected := make([]int, 16)
	for i := range expected {
		expected[i] = local.Int()
	}

	assert.Equal(t, expected, dest[:])
}

func TestStruct(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	type inner struct {
		Public  []int
		private []int
	}

	var dest inner

	Provide(t, &dest)

	assert.Nil(t, dest.private)
	assert.Len(t, dest.Public, 1)
	assert.Equal(t, dest.Public[0], local.Int())
}

func TestNestedStruct(t *testing.T) {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	local := rand.New(rand.NewSource(seed))

	type nested struct {
		Ints []int
	}

	type inner struct {
		Public  *nested
		Public2 nested
		private *nested
	}

	var dest inner

	Provide(t, &dest)
	assert.NotNil(t, dest.Public)
	assert.Nil(t, dest.private)

	assert.Len(t, dest.Public.Ints, 1)
	assert.Len(t, dest.Public2.Ints, 1)

	assert.Equal(t, dest.Public.Ints[0], local.Int())
	assert.Equal(t, dest.Public2.Ints[0], local.Int())
}
