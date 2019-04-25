package types

import (
	"fmt"
	"math/big"
	"testing"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/stretchr/testify/assert"
)

func MustBigToFixed(b *big.Float, t *testing.T) uint64 {
	ret, err := BigToFixed(b)
	assert.NoError(t, err)
	return ret
}

func MustFixedToBig(f uint64, t *testing.T) *big.Float {
	ret, err := FixedToBig(f)
	assert.NoError(t, err)
	return ret
}

func TestBigToFixed(t *testing.T) {
	tf.UnitTest(t)

	t.Run("truncate decimal", func(t *testing.T) {
		x := 30004828.209239083240324
		bigX := big.NewFloat(x)
		assert.Equal(t, uint64(30004828209), MustBigToFixed(bigX, t))
	})

	t.Run("upper limit", func(t *testing.T) {
		y := "18014398509481983.555"
		bigY := new(big.Float)
		_, _, err := bigY.Parse(y, 10)
		assert.NoError(t, err)
		assert.Equal(t, uint64(18014398509481983555), MustBigToFixed(bigY, t))
	})

	t.Run("handle high precision decimal", func(t *testing.T) {
		z := 300.0000000000000000000000000000001
		bigZ := big.NewFloat(z)
		assert.Equal(t, uint64(300000), MustBigToFixed(bigZ, t))
	})

	t.Run("no truncation", func(t *testing.T) {
		w := 4000001.530
		bigW := big.NewFloat(w)
		assert.Equal(t, uint64(4000001530), MustBigToFixed(bigW, t))
	})
}

func TestFixedToBig(t *testing.T) {
	tf.UnitTest(t)

	t.Run("whole number", func(t *testing.T) {
		x := uint64(81053000)
		expectedBigX := big.NewFloat(81053.0)
		// Use strings to forget about precison + rounding error
		expectedBigXStr := fmt.Sprintf("%.3f", expectedBigX)                        // nolint: govet
		assert.Equal(t, expectedBigXStr, fmt.Sprintf("%.3f", MustFixedToBig(x, t))) // nolint: govet
	})

	t.Run("fractional part and whole number", func(t *testing.T) {
		y := uint64(75123499)
		expectedBigY := big.NewFloat(75123.499)
		expectedBigYStr := fmt.Sprintf("%.3f", expectedBigY)                        // nolint: govet
		assert.Equal(t, expectedBigYStr, fmt.Sprintf("%.3f", MustFixedToBig(y, t))) // nolint: govet
	})
}

func TestFixedRoundTrip(t *testing.T) {
	tf.UnitTest(t)

	w := 4000001.530
	bigW := big.NewFloat(w)
	expectedWStr := fmt.Sprintf("%.3f", bigW)                                                      // nolint: govet
	assert.Equal(t, expectedWStr, fmt.Sprintf("%.3f", MustFixedToBig(MustBigToFixed(bigW, t), t))) // nolint: govet
}

func TestOversized(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Oversized *big.Float", func(t *testing.T) {
		a := "18014398509481986.555"
		bigA := new(big.Float)
		_, _, err := bigA.Parse(a, 10)
		assert.NoError(t, err)
		_, err = BigToFixed(bigA)
		assert.Error(t, err)
	})

	// Oversized uint64
	t.Run("Oversized uint64", func(t *testing.T) {
		b := uint64(18014398509481986555)
		_, err := FixedToBig(b)
		assert.Error(t, err)
	})
}
