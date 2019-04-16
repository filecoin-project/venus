package proofs

import (
	"bytes"
	"sort"

	"github.com/filecoin-project/go-filecoin/types"
)

// SortedCommRs is a slice of CommRs that has deterministic ordering.
type SortedCommRs struct {
	c []types.CommR
}

// NewSortedCommRs returns a SortedCommRs with the given SortedCommRs
func NewSortedCommRs(commRs ...types.CommR) SortedCommRs {
	fn := func(i, j int) bool {
		return bytes.Compare(commRs[i][:], commRs[j][:]) == -1
	}

	sort.Slice(commRs[:], fn)

	return SortedCommRs{
		c: commRs,
	}
}

// Values returns the sorted SortedCommRs as a slice
func (s *SortedCommRs) Values() []types.CommR {
	return s.c
}
