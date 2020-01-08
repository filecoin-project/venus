package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type prodRndSource struct {
}

// NewProdRandomnessSource crates a production randomness source.
func NewProdRandomnessSource() RandomnessSource {
	return &prodRndSource{}
}

var _ RandomnessSource = (*prodRndSource)(nil)

func (*prodRndSource) Randomness(epoch types.BlockHeight, offset uint64) runtime.Randomness {
	// TODO: implement randomness based on new spec (issue: #3717)
	return []byte{}
}
