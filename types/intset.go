package types

import (
	"github.com/golang-collections/go-datastructures/bitarray"
)

// IntSet is a space-efficient set of uint64
type IntSet struct {
	ba bitarray.BitArray
}

// NewIntSet returns an new IntSet, optionally initialized with integers
func NewIntSet(ints ...uint64) IntSet {
	out := IntSet{ba: bitarray.NewSparseBitArray()}
	for _, i := range ints {
		out.Add(i)
	}
	return out
}

// Add adds an integer to this IntSet
func (is IntSet) Add(i uint64) {
	// Ignoring errors as we are using SparseBitArrays, which never return errors
	_ = is.ba.SetBit(i)
}

// Contains returns whether an integer exists in this IntSet
func (is IntSet) Contains(i uint64) bool {
	// Ignoring errors as we are using SparseBitArrays, which never return errors
	isSet, _ := is.ba.GetBit(i)
	return isSet
}

// Union returns a new IntSet, the result of a set union of the receiver and other
func (is IntSet) Union(other IntSet) IntSet {
	return IntSet{ba: is.ba.Or(other.ba)}
}

// Intersection returns a new IntSet, which is the intersection of the receiver and other
func (is IntSet) Intersection(other IntSet) IntSet {
	return IntSet{ba: is.ba.And(other.ba)}
}

// Difference returns a new IntSet, containing values in the receiver that are not in other
func (is IntSet) Difference(other IntSet) IntSet {
	out := NewIntSet()

	for _, i := range is.ba.ToNums() {
		if !other.Contains(i) {
			out.Add(i)
		}
	}

	return out
}

// Integers returns a slice with all integers in this IntSet
func (is IntSet) Integers() []uint64 {
	return is.ba.ToNums()
}
