package types

import (
	"fmt"
	"math/big"
	"testing"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/assert"
)

func MustBigToFixed(b *big.Float, a *assert.Assertions) uint64 {
	ret, err := BigToFixed(b)
	a.NoError(err)
	return ret
}

func MustFixedToBig(f uint64, a *assert.Assertions) *big.Float {
	ret, err := FixedToBig(f)
	a.NoError(err)
	return ret
}

func TestBigToFixed(t *testing.T) {
	assert := assert.New(t)
	t.Run("truncate decimal", func(t *testing.T) {
		x := 30004828.209239083240324
		bigX := big.NewFloat(x)
		assert.Equal(uint64(30004828209), MustBigToFixed(bigX, assert))
	})

	t.Run("upper limit", func(t *testing.T) {
		y := "18014398509481983.555"
		bigY := new(big.Float)
		bigY.Parse(y, 10)
		assert.Equal(uint64(18014398509481983555), MustBigToFixed(bigY, assert))
	})

	t.Run("handle high precision decimal", func(t *testing.T) {
		z := 300.0000000000000000000000000000001
		bigZ := big.NewFloat(z)
		assert.Equal(uint64(300000), MustBigToFixed(bigZ, assert))
	})

	t.Run("no truncation", func(t *testing.T) {
		w := 4000001.530
		bigW := big.NewFloat(w)
		assert.Equal(uint64(4000001530), MustBigToFixed(bigW, assert))
	})
}

func TestFixedToBig(t *testing.T) {
	assert := assert.New(t)
	t.Run("whole number", func(t *testing.T) {
		x := uint64(81053000)
		expectedBigX := big.NewFloat(81053.0)
		// Use strings to forget about precison + rounding error
		expectedBigXStr := fmt.Sprintf("%.3f", expectedBigX)
		assert.Equal(expectedBigXStr, fmt.Sprintf("%.3f", MustFixedToBig(x, assert)))
	})

	t.Run("fractional part and whole number", func(t *testing.T) {
		y := uint64(75123499)
		expectedBigY := big.NewFloat(75123.499)
		expectedBigYStr := fmt.Sprintf("%.3f", expectedBigY)
		assert.Equal(expectedBigYStr, fmt.Sprintf("%.3f", MustFixedToBig(y, assert)))
	})
}

func TestFixedRoundTrip(t *testing.T) {
	assert := assert.New(t)
	w := 4000001.530
	bigW := big.NewFloat(w)
	expectedWStr := fmt.Sprintf("%.3f", bigW)
	assert.Equal(expectedWStr, fmt.Sprintf("%.3f", MustFixedToBig(MustBigToFixed(bigW, assert), assert)))
}

func TestOversized(t *testing.T) {
	assert := assert.New(t)

	t.Run("Oversized *big.Float", func(t *testing.T) {
		a := "18014398509481986.555"
		bigA := new(big.Float)
		bigA.Parse(a, 10)
		_, err := BigToFixed(bigA)
		assert.Error(err)
	})

	// Oversized uint64
	t.Run("Oversized uint64", func(t *testing.T) {
		b := uint64(18014398509481986555)
		_, err := FixedToBig(b)
		assert.Error(err)
	})
}
