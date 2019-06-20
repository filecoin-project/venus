package rleplus

import (
	"encoding/binary"
	"sort"

	"github.com/filecoin-project/go-filecoin/rleplus/internal"
)

// Encode returns the RLE+ representation of the provided integers.
// Also returned is the number of bits required by this encoding,
// which is not necessarily on a byte boundary.
//
// The RLE+ spec is here: https://github.com/filecoin-project/specs/blob/master/data-structures.md#rle-bitset-encoding
// and is described by the BNF Grammar:
//
//    <encoding> ::= <header> <blocks>
//    <header> ::= <version> <bit>
//    <version> ::= "00"
//    <blocks> ::= <block> <blocks> | ""
//    <block> ::= <block_single> | <block_short> | <block_long>
//    <block_single> ::= "1"
//    <block_short> ::= "01" <bit> <bit> <bit> <bit>
//    <block_long> ::= "00" <unsigned_varint>
//    <bit> ::= "0" | "1"
//
// The encoding is returned as a []byte, each byte packed starting with the low-order bit (LSB0)
func Encode(ints []uint64) ([]byte, int) {
	v := bitvector.BitVector{BytePacking: bitvector.LSB0}
	firstBit, runs := RunLengths(ints)

	// Add version
	v.Push(0)
	v.Push(0)

	v.Push(firstBit)

	for _, run := range runs {
		switch {
		case run == 1:
			v.Push(1)
		case run < 16:
			v.Push(0)
			v.Push(1)
			v.Extend(byte(run), 4, bitvector.LSB0)
		case run >= 16:
			v.Push(0)
			v.Push(0)
			buf := make([]byte, 10)
			numBytes := binary.PutUvarint(buf, run)
			for i := 0; i < numBytes; i++ {
				v.Extend(buf[i], 8, bitvector.LSB0)
			}
		}
	}

	return v.Buf, v.Len
}

// Decode returns integers represented by the given RLE+ encoding
//
// The length of the encoding is not specified.  It is inferred by
// reading zeroes from the (possibly empty) BitVector, by virtue
// of the behavior of BitVector.Take returning 0 when the end of
// the BitVector has been reached. This has the downside of not
// being able to detect corrupt encodings.
//
// The passed []byte should be packed in LSB0 bit numbering
func Decode(buf []byte) (ints []uint64) {
	if len(buf) == 0 {
		return
	}

	v := bitvector.NewBitVector(buf, bitvector.LSB0)
	take := v.Iterator(bitvector.LSB0)

	// Read version and discard
	take(1)
	take(1)

	curIdx := uint64(0)
	curBit := take(1)
	var runLength int
	done := false

	for done == false {
		y := take(1)
		switch y {
		case 1:
			runLength = 1
		case 0:
			val := take(1)

			if val == 1 {
				// short block
				runLength = int(take(4))
			} else {
				// long block
				var buf []byte
				for {
					b := take(8)
					buf = append(buf, b)
					if b&0x80 == 0 {
						break
					}
				}
				x, _ := binary.Uvarint(buf)

				if x == 0 {
					done = true
				}
				runLength = int(x)
			}
		}

		if curBit == 1 {
			for j := 0; j < runLength; j++ {
				ints = append(ints, curIdx+uint64(j))
			}
		}
		curIdx += uint64(runLength)
		curBit = 1 - curBit
	}

	return
}

// RunLengths transforms integers into its bit-set-run-length representation.
//
// A set of unsigned integers { 0, 2, 4, 5, 6 } can be thought of as
// indices into a bitset { 1, 0, 1, 0, 1, 1, 1 } where bitset[index] == 1.
//
// The bit set run lengths of this set would then be { 1, 1, 1, 1, 3 },
// representing lengths of runs alternating between 1 and 0, starting
// with a first bit of 1.
//
// This is a helper function for Encode()
func RunLengths(ints []uint64) (firstBit byte, runs []uint64) {
	if len(ints) == 0 {
		return
	}

	// Sort our incoming numbers
	sort.Slice(ints, func(i, j int) bool { return ints[i] < ints[j] })

	last := ints[0]

	// Initialize our return value
	if last == 0 {
		firstBit = 1
	}

	if firstBit == 0 {
		// first run of zeroes
		runs = append(runs, last)
	}
	runs = append(runs, 1)

	for _, cur := range ints[1:] {
		delta := cur - last
		switch {
		case delta == 1:
			runs[len(runs)-1]++
		case delta > 1:
			// add run of zeroes if there is a gap
			runs = append(runs, delta-1)
			runs = append(runs, 1)
		default:
			// repeated number?
		}
		last = cur
	}
	return
}
