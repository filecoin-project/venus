package rleplus_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/filecoin-project/go-filecoin/rleplus"
	"github.com/filecoin-project/go-filecoin/rleplus/internal"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"gotest.tools/assert"
)

func TestRleplus(t *testing.T) {
	tf.UnitTest(t)

	t.Run("Encode", func(t *testing.T) {
		// Encode an intset
		ints := []uint64{
			// run of 1
			0,
			// gap of 1
			// run of 1
			2,
			// gap of 1
			// run of 3
			4, 5, 6,
			// gap of 4
			// run of 17
			11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27,
		}

		expectedBits := []byte{
			//0, 0, // version
			1,                // first bit
			1,                // run of 1
			1,                // gap of 1
			1,                // run of 1
			1,                // gap of 1
			0, 1, 1, 1, 0, 0, // run of 3
			0, 1, 0, 0, 1, 0, // gap of 4

			// run of 17 < 0 0 (varint) >
			0, 0,
			1, 0, 0, 0, 1, 0, 0, 0,
		}

		v := bitvector.BitVector{}
		for _, bit := range expectedBits {
			v.Push(bit)
		}
		actualBytes, _ := rleplus.Encode(ints)

		for idx, expected := range v.Buf {
			assert.Equal(
				t,
				fmt.Sprintf("%08b", expected),
				fmt.Sprintf("%08b", actualBytes[idx]),
			)
		}
	})

	t.Run("Encode Decode Symmetry", func(t *testing.T) {
		testCases := [][]uint64{
			{},
			{1},
			{0},
			{0, 1, 2, 3},
			{
				// run of 1
				0,
				// gap of 1
				// run of 1
				2,
				// gap of 1
				// run of 3
				4, 5, 6,
				// gap of 4
				// run of 17
				11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27,
			},
		}

		for _, tc := range testCases {
			encoded, _ := rleplus.Encode(tc)
			result := rleplus.Decode(encoded)

			sort.Slice(tc, func(i, j int) bool { return tc[i] < tc[j] })
			sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })

			assert.Equal(t, len(tc), len(result))

			for idx, expected := range tc {
				assert.Equal(t, expected, result[idx])
			}
		}
	})

	t.Run("Outputs same as reference implementation", func(t *testing.T) {
		// Encoding bitvec![LittleEndian; 1, 0, 1, 0, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1]
		// in the Rust reference implementation gives an encoding of [223, 145, 136, 0]
		// The bit vector is equivalent to the integer set { 0, 2, 4, 5, 6, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27 }
		referenceEncoding := []byte{223, 145, 136, 0}
		expectedNumbers := []uint64{0, 2, 4, 5, 6, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27}

		encoded, _ := rleplus.Encode(expectedNumbers)

		// Our encoded bytes are the same as the ref bytes
		assert.Equal(t, len(referenceEncoding), len(encoded))
		for idx, expected := range referenceEncoding {
			assert.Equal(t, expected, encoded[idx])
		}

		decoded := rleplus.Decode(referenceEncoding)

		// Our decoded integers are the same as expected
		sort.Slice(decoded, func(i, j int) bool { return decoded[i] < decoded[j] })
		assert.Equal(t, len(expectedNumbers), len(decoded))
		for idx, expected := range expectedNumbers {
			assert.Equal(t, expected, decoded[idx])
		}
	})

	t.Run("RunLengths", func(t *testing.T) {
		testCases := []struct {
			ints  []uint64
			first byte
			runs  []uint64
		}{
			// empty
			{},

			// leading with ones
			{[]uint64{0}, 1, []uint64{1}},
			{[]uint64{0, 1}, 1, []uint64{2}},
			{[]uint64{0, 0xffffffff, 0xffffffff + 1}, 1, []uint64{1, 0xffffffff - 1, 2}},

			// leading with zeroes
			{[]uint64{1}, 0, []uint64{1, 1}},
			{[]uint64{2}, 0, []uint64{2, 1}},
			{[]uint64{10, 11, 13, 20}, 0, []uint64{10, 2, 1, 1, 6, 1}},
			{[]uint64{10, 11, 11, 13, 20, 10, 11, 13, 20}, 0, []uint64{10, 2, 1, 1, 6, 1}},
		}

		for _, testCase := range testCases {
			first, runs := rleplus.RunLengths(testCase.ints)
			assert.Equal(t, testCase.first, first)
			assert.Equal(t, len(testCase.runs), len(runs))
			for idx, runLength := range testCase.runs {
				assert.Equal(t, runLength, runs[idx])
			}
		}
	})
}
