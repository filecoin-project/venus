package types

import (
	"bytes"
	"github.com/filecoin-project/go-filecoin/proofs"
	"sort"
)

// SortedCommRSlice is a slice of CommRs that has deterministic ordering.
type SortedCommRSlice struct {
	c []proofs.CommR
}

// NewSortedCommRSlice returns a SortedCommRSlice with the given CommRs
func NewSortedCommRSlice(commRs ...proofs.CommR) SortedCommRSlice {
	fn := func(i, j int) bool {
		return bytes.Compare(commRs[i][:], commRs[j][:]) == -1
	}

	sort.Slice(commRs[:], fn)

	return SortedCommRSlice{
		c: commRs,
	}
}

// Values returns the sorted CommRs as a slice
func (s *SortedCommRSlice) Values() []proofs.CommR {
	return s.c
}
