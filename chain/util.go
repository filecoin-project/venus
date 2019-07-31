package chain

import (
	"github.com/filecoin-project/go-filecoin/types"
)

// Reverse reverses the order of the slice `chain`.
func Reverse(chain []types.TipSet) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(chain)/2 - 1; i >= 0; i-- {
		opp := len(chain) - 1 - i
		chain[i], chain[opp] = chain[opp], chain[i]
	}
}
