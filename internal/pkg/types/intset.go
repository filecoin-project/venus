package types

import (
	"fmt"

	"github.com/Workiva/go-datastructures/bitarray"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/rleplus"
	"github.com/polydawn/refmt/obj/atlas"
)

func init() {
	encoding.RegisterIpldCborType(intSetAtlasEntry)
}

var intSetAtlasEntry = atlas.BuildEntry(IntSet{}).Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(
		func(is IntSet) ([]byte, error) {
			bytes, _, err := rleplus.Encode(is.Values())
			return bytes, err
		})).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(
		func(x []byte) (IntSet, error) {
			ints, err := rleplus.Decode(x)
			if err != nil {
				return EmptyIntSet(), err
			}
			return NewIntSet(ints...), nil
		})).
	Complete()

// IntSet is a space-efficient set of uint64
type IntSet struct {
	ba bitarray.BitArray
}

// NewIntSet returns a new IntSet, optionally initialized with integers
func NewIntSet(ints ...uint64) IntSet {
	// We are ignoring errors from SetBit, GetBit since SparseBitArrays never return errors for those methods
	out := IntSet{ba: bitarray.NewSparseBitArray()}
	for _, i := range ints {
		_ = out.ba.SetBit(i)
	}
	return out
}

// Has returns whether an integer exists in this IntSet
func (is IntSet) Has(i uint64) bool {
	isSet, _ := is.ba.GetBit(i)
	return isSet
}

// HasSubset returns true if every element in other is also in the receiver
func (is IntSet) HasSubset(other IntSet) bool {
	// The method we're calling here seems weird, but in bitarray (https://github.com/Workiva/go-datastructures/blob/f07cbe3f82ca2fd6e5ab94afce65fe43319f675f/bitarray/block.go#L97)
	// "intersect" means, "wholly contained within"
	return is.ba.Intersects(other.ba)
}

// Add returns a new IntSet, the result of adding i to the receiver
func (is IntSet) Add(i uint64) IntSet {
	return is.Union(NewIntSet(i))
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
		if !other.Has(i) {
			_ = out.ba.SetBit(i)
		}
	}

	return out
}

// Values returns a slice with all integers in this IntSet
func (is IntSet) Values() []uint64 {
	return is.ba.ToNums()
}

// String returns a printable string of the IntSet.
func (is IntSet) String() string {
	return fmt.Sprintf("%d", is.Values())
}

// Size returns the size of an IntSet.  It should be more efficient than
// len(is.Values()).
func (is IntSet) Size() int {
	var size int
	it := is.ba.Blocks()
	// it.Next() must be called to point it at the first block
	for it.Next() {
		_, bitBlock := it.Value()
		bm := bitarray.Bitmap64(bitBlock)
		size += bm.PopCount()
	}
	return size
}

// EmptyIntSet returns an empty IntSet.
func EmptyIntSet() IntSet {
	return IntSet{ba: bitarray.NewSparseBitArray()}
}
