package vmcontext

import (
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type prodRndSource struct {
}

// NewProdRandomnessSource crates a production randomness source.
func NewProdRandomnessSource() randomnessSource {
	return &prodRndSource{}
}

var _ randomnessSource = (*prodRndSource)(nil)

func (*prodRndSource) Randomness(epoch types.BlockHeight, offset uint64) runtime.Randomness {
	// TODO: implement when spec is ready
	return []byte{}
}
