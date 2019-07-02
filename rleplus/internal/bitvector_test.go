package bitvector_test

import (
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-filecoin/rleplus/internal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"gotest.tools/assert"
)

// Note: When using this helper assertion, expectedBits should *only* be 0s and 1s.
func assertBitVector(t *testing.T, expectedBits []byte, actual bitvector.BitVector) {
	assert.Equal(t, len(expectedBits), actual.Len)

	for idx, bit := range expectedBits {
		actualBit, err := actual.Get(idx)
		assert.NilError(t, err)
		assert.Equal(t, bit, actualBit)
	}
}

func TestBitVector(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Zero Value", func(t *testing.T) {
		var v bitvector.BitVector

		assert.Equal(t, bitvector.LSB0, v.BytePacking)
	})

	t.Run("Push", func(t *testing.T) {
		// MSB0 bit numbering
		v := bitvector.BitVector{BytePacking: bitvector.MSB0}
		v.Push(1)
		v.Push(0)
		v.Push(1)
		v.Push(1)

		assert.Equal(t, byte(176), v.Buf[0])

		// LSB0 bit numbering
		v = bitvector.BitVector{BytePacking: bitvector.LSB0}
		v.Push(1)
		v.Push(0)
		v.Push(1)
		v.Push(1)

		assert.Equal(t, byte(13), v.Buf[0])
	})

	t.Run("Get", func(t *testing.T) {
		bits := []byte{1, 0, 1, 1, 0, 0, 1, 0}

		for _, numbering := range []bitvector.BitNumbering{bitvector.MSB0, bitvector.LSB0} {
			v := bitvector.BitVector{BytePacking: numbering}

			for _, bit := range bits {
				v.Push(bit)
			}

			for idx, expected := range bits {
				actual, _ := v.Get(idx)
				assert.Equal(t, expected, actual)
			}
		}
	})

	t.Run("Extend", func(t *testing.T) {
		val := byte(171) // 0b10101011

		var v bitvector.BitVector

		// MSB0 bit numbering
		v = bitvector.BitVector{}
		v.Extend(val, 4, bitvector.MSB0)
		assertBitVector(t, []byte{1, 0, 1, 0}, v)
		v.Extend(val, 5, bitvector.MSB0)
		assertBitVector(t, []byte{1, 0, 1, 0, 1, 0, 1, 0, 1}, v)

		// LSB0 bit numbering
		v = bitvector.BitVector{}
		v.Extend(val, 4, bitvector.LSB0)
		assertBitVector(t, []byte{1, 1, 0, 1}, v)
		v.Extend(val, 5, bitvector.LSB0)
		assertBitVector(t, []byte{1, 1, 0, 1, 1, 1, 0, 1, 0}, v)
	})

	t.Run("Take", func(t *testing.T) {
		var v bitvector.BitVector

		bits := []byte{1, 0, 1, 0, 1, 0, 1, 1}
		for _, bit := range bits {
			v.Push(bit)
		}

		assert.Equal(t, byte(176), v.Take(4, 4, bitvector.MSB0))
		assert.Equal(t, byte(13), v.Take(4, 4, bitvector.LSB0))
	})

	t.Run("Iterator", func(t *testing.T) {
		var v bitvector.BitVector

		for i := 0; i < 256; i++ {
			v.Push(byte(rand.Int() & 1))
		}

		next := v.Iterator(bitvector.LSB0)

		// compare to Get()
		for i := 0; i < v.Len; i++ {
			expected, _ := v.Get(i)
			assert.Equal(t, expected, next(1))
		}

		// out of range should return zero
		assert.Equal(t, byte(0), next(1))
		assert.Equal(t, byte(0), next(8))

		// compare to Take()
		next = v.Iterator(bitvector.LSB0)
		assert.Equal(t, next(5), v.Take(0, 5, bitvector.LSB0))
		assert.Equal(t, next(8), v.Take(5, 8, bitvector.LSB0))
	})
}
