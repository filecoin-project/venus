package porcelain

import (
	"github.com/filecoin-project/go-filecoin/types"
)

type chBlockHeightPlumbing interface {
	ChainHead() types.TipSet
}

// ChainBlockHeight determines the current block height
func ChainBlockHeight(plumbing chBlockHeightPlumbing) (*types.BlockHeight, error) {
	currentHeight, err := plumbing.ChainHead().Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(currentHeight), nil
}
