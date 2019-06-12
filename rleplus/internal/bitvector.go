package bitvector

import "errors"

var (
	// ErrOutOfRange - the index passed is out of range for the BitVector
	ErrOutOfRange = errors.New("index out of range")
)

// BitNumbering indicates the ordering of bits, either
// least-significant bit in position 0, or most-significant bit
// in position 0.
//
// It it used in 3 ways with BitVector:
// 1. Ordering of bits within the Buf []byte structure
// 2. What order to add bits when using Extend()
// 3. What order to read bits when using Take()
//
// https://en.wikipedia.org/wiki/Bit_numbering
type BitNumbering int

const (
	// LSB0 - bit ordering starts with the low-order bit
	LSB0 BitNumbering = iota

	// MSB0 - bit ordering starts with the high-order bit
	MSB0
)

// BitVector is used to manipulate ordered collections of bits
type BitVector struct {
	BytePacking BitNumbering
	Buf         []byte
	Len         int
}

// NewBitVector constructs a new BitVector from a slice of bytes.
//
// The bytePacking parameter is required to know how to interpret the
// bit ordering within the bytes.
func NewBitVector(buf []byte, bytePacking BitNumbering) *BitVector {
	return &BitVector{
		BytePacking: bytePacking,
		Buf:         buf,
		Len:         len(buf) * 8,
	}
}

// Push adds a single bit to the BitVector.
//
// Although it takes a byte, only the low-order
// bit is used, so just use 0 or 1.
func (v *BitVector) Push(val byte) {
	if v.Len%8 == 0 {
		v.Buf = append(v.Buf, 0)
	}

	switch v.BytePacking {
	case LSB0:
		v.pushLSB0(val)
	default:
		v.pushMSB0(val)
	}

	v.Len++
}

// Get returns byte either 0, 1
func (v *BitVector) Get(idx int) (byte, error) {
	if idx >= v.Len {
		return 0, ErrOutOfRange
	}

	switch v.BytePacking {
	case LSB0:
		return v.getLSB0(idx), nil
	default:
		return v.getMSB0(idx), nil
	}
}

// Extend adds up to 8 bits to the receiver, in the order indicated
//
// Given a byte b == 0b11010101
// v.Extend(b, 4, LSB0) would add <1, 0, 1, 0>
// v.Extend(b, 4, MSB0) would add <1, 1, 0, 1>
func (v *BitVector) Extend(val byte, count uint, order BitNumbering) {
	if count > 8 {
		count = 8
	}

	switch order {
	case LSB0:
		v.extendLSB0(val, count)
	default:
		v.extendMSB0(val, count)
	}
}

// Take reads up to 8 bits at the given index, and returns them in a byte.
// The byte packing is determined by the order parameter.
//
// Given a BitVector < 1, 1, 0, 1, 0, 1, 0, 1 >
// v.Take(1, 4, LSB0) would return 0b00001011
// v.Take(1, 4, MSB0) would return 0b00001101
func (v *BitVector) Take(index int, count int, order BitNumbering) byte {
	if count > 8 {
		count = 8
	}

	switch order {
	case LSB0:
		return v.takeLSB0(index, count)
	default:
		return v.takeMSB0(index, count)
	}
}

// Iterator returns a function, which when invokes, returns the number
// of bits passed, and increments an internal cursor.
//
// When the end of the BitVector is reached, it returns zeroes indefinitely
func (v *BitVector) Iterator(order BitNumbering) func(int) byte {
	cursor := 0
	return func(count int) (out byte) {
		out = v.Take(cursor, count, order)
		cursor += count
		return
	}
}

func (v *BitVector) getLSB0(idx int) byte {
	blockIdx := idx / 8
	return v.Buf[blockIdx] >> uint(idx%8) & 1
}

func (v *BitVector) getMSB0(idx int) byte {
	blockIdx := idx / 8
	return v.Buf[blockIdx] >> uint(7-idx%8) & 1
}

func (v *BitVector) pushLSB0(val byte) {
	lastIdx := v.Len / 8
	v.Buf[lastIdx] |= (val & 1) << uint(v.Len%8)
}

func (v *BitVector) pushMSB0(val byte) {
	lastIdx := v.Len / 8
	v.Buf[lastIdx] |= (val & 1) << uint(7-(v.Len%8))
}

// extendLSB0 pushes bits onto the BitVector, low-order bit first
// e.g. v.extendLSB0(171, 4) // adds <...., 1, 1, 0, 1> since 171 == 0b10101011
func (v *BitVector) extendLSB0(val byte, count uint) {
	for i := uint(0); i < count; i++ {
		v.Push((val >> i) & 1)
	}
}

// takeLSB0 fetches up to 8 bits and returns it as a byte, low-order bit first
// e.g. BitVector <1, 1, 0, 1, 0, 1, 0, 1 >, v.takeLSB0(1, 4) returns 0b00001011
func (v *BitVector) takeLSB0(index int, count int) (out byte) {
	for i := 0; i < count; i++ {
		val, _ := v.Get(index + i)
		out |= val << uint(i)
	}
	return
}

// extendMSB0 pushes up to 8 bits onto the BitVector, high-order bit first
// e.g. v.extendMSB0(171, 4) // adds <...., 1, 0, 1, 0> since 171 == 0b10101011
func (v *BitVector) extendMSB0(val byte, count uint) {
	for i := uint(0); i < count; i++ {
		v.Push((val >> (7 - i)) & 1)
	}
}

// takeMSB0 fetches up to 8 bits and returns it as a byte, high-order bit first
// e.g. BitVector <1, 1, 0, 1, 0, 1, 0, 1 >, v.takeMSB0(1, 4) returns 0b00001101
func (v *BitVector) takeMSB0(index int, count int) (out byte) {
	for i := 0; i < count; i++ {
		// 3, 2, 1, 0
		val, _ := v.Get(index + i)
		out |= val << uint(count-i-1)
	}
	return
}
