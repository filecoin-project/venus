package testutil

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func getRand() *rand.Rand {
	seed := time.Now().UnixNano()
	rand.Seed(seed)
	return rand.New(rand.NewSource(seed))
}

func TestDefaultBytes(t *testing.T) {
	local := getRand()

	for i := 0; i < 16; i++ {
		var b []byte
		Provide(t, &b)
		require.Len(t, b, defaultBytesFixedSize)

		expected := make([]byte, defaultBytesFixedSize)
		local.Read(expected[:])
		require.Equal(t, expected, b)
	}

}

func TestDefaultString(t *testing.T) {
	local := getRand()

	for i := 0; i < 16; i++ {
		var s string
		Provide(t, &s)
		require.Len(t, s, defaultBytesFixedSize*2)

		expected := make([]byte, defaultBytesFixedSize)
		local.Read(expected[:])
		require.Equal(t, hex.EncodeToString(expected), s)
	}
}

func TestDefaultInt(t *testing.T) {
	local := getRand()

	for i := 0; i < 16; i++ {
		var n int
		Provide(t, &n)
		require.Equal(t, n, local.Int())
	}
}

func TestDefaultInt64(t *testing.T) {
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(int64(0))))

	local := getRand()

	for i := 0; i < 16; i++ {
		var n int64
		Provide(t, &n)
		require.Equal(t, n, int64(local.Int()))
	}
}

func TestDefaultInt32(t *testing.T) {
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(int32(0))))

	local := getRand()

	for i := 0; i < 16; i++ {
		var n int32
		Provide(t, &n)
		require.Equal(t, n, int32(local.Int()))
	}
}

func TestDefaultFloat64(t *testing.T) {
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(float64(0))))

	local := getRand()

	for i := 0; i < 16; i++ {
		var n float64
		Provide(t, &n)
		require.Equal(t, n, float64(local.Int()))
	}
}

func TestDefaultIntType(t *testing.T) {
	type number int
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(number(0))))

	local := getRand()

	for i := 0; i < 16; i++ {
		var n number
		Provide(t, &n)
		require.Equal(t, n, number(local.Int()))
	}
}

func TestDefaultFloatType(t *testing.T) {
	type double float64
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(double(0))))

	local := getRand()

	for i := 0; i < 16; i++ {
		var n double
		Provide(t, &n)
		require.Equal(t, n, double(local.Int()))
	}
}

func TestDefaultIntSlice(t *testing.T) {
	local := getRand()

	var dest []int
	Provide(t, &dest)

	require.Len(t, dest, 1)
	require.Equal(t, dest[0], local.Int())
}

func TestDefaultIntSliceWithLen(t *testing.T) {
	local := getRand()

	var dest []int
	Provide(t, &dest, WithSliceLen(10))

	require.Len(t, dest, 10)
	for i := range dest {
		require.Equal(t, dest[i], local.Int())
	}
}

func TestDefaultIntTypeSlice(t *testing.T) {
	type number int
	require.False(t, defaultValueProviderRegistry.has(reflect.TypeOf(number(0))))

	local := getRand()

	var dest []number
	Provide(t, &dest)

	require.Len(t, dest, 1)
	require.Equal(t, dest[0], number(local.Int()))
}

func TestDefaultNonNilIntSlice(t *testing.T) {
	local := getRand()

	dest := make([]int, 16)
	Provide(t, &dest)

	expected := make([]int, 16)
	for i := range expected {
		expected[i] = local.Int()
	}

	require.Equal(t, expected, dest)
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

	require.Equal(t, expected, dest)
}

func TestIntSliceRanged(t *testing.T) {
	min := 10
	max := 20

	dest := make([]int, 256)
	Provide(t, &dest, IntRangedProvider(min, max))

	for i := range dest {
		require.GreaterOrEqual(t, dest[i], min)
		require.Less(t, dest[i], max)
	}
}

func TestNegativeIntSliceRanged(t *testing.T) {
	min := -20
	max := -10

	dest := make([]int, 256)
	Provide(t, &dest, IntRangedProvider(min, max))

	for i := range dest {
		require.GreaterOrEqual(t, dest[i], min)
		require.Less(t, dest[i], max)
	}
}

func TestDefaultIntArray(t *testing.T) {
	local := getRand()

	var dest [16]int
	Provide(t, &dest)

	expected := make([]int, 16)
	for i := range expected {
		expected[i] = local.Int()
	}

	require.Equal(t, expected, dest[:])
}

func TestStruct(t *testing.T) {
	local := getRand()

	type inner struct {
		Public  []int
		private []int
	}

	var dest inner

	Provide(t, &dest)

	require.Nil(t, dest.private)
	require.Len(t, dest.Public, 1)
	require.Equal(t, dest.Public[0], local.Int())
}

func TestNestedStruct(t *testing.T) {
	local := getRand()

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
	require.NotNil(t, dest.Public)
	require.Nil(t, dest.private)

	require.Len(t, dest.Public.Ints, 1)
	require.Len(t, dest.Public2.Ints, 1)

	require.Equal(t, dest.Public.Ints[0], local.Int())
	require.Equal(t, dest.Public2.Ints[0], local.Int())
}
