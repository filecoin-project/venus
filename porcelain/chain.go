package porcelain

import (
	"github.com/filecoin-project/go-filecoin/types"
)

type chBlockHeightPlumbing interface {
	ChainHead() (*types.TipSet, error)
}

// ChainBlockHeight determines the current block height
func ChainBlockHeight(plumbing chBlockHeightPlumbing) (*types.BlockHeight, error) {
	head, err := plumbing.ChainHead()
	if err != nil {
		return nil, err
	}
	height, err := head.Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(height), nil
}
