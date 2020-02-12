package vmcontext

import (
	"github.com/filecoin-project/specs-actors/actors/abi"
)

type prodRndSource struct {
}

// NewProdRandomnessSource crates a production randomness source.
func NewProdRandomnessSource() RandomnessSource {
	return &prodRndSource{}
}

var _ RandomnessSource = (*prodRndSource)(nil)

func (*prodRndSource) Randomness(epoch abi.ChainEpoch) abi.RandomnessSeed {
	// TODO: implement randomness based on new spec (issue: #3717)
	return []byte{}
}
