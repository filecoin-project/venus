package porcelain

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/types"
)

type chPlumbing interface {
	ChainLs(ctx context.Context) <-chan interface{}
}

// ChainBlockHeight determines the current block height
func ChainBlockHeight(ctx context.Context, plumbing chPlumbing) (*types.BlockHeight, error) {
	lsCtx, cancelLs := context.WithCancel(ctx)
	tipSetCh := plumbing.ChainLs(lsCtx)
	head := <-tipSetCh
	cancelLs()

	if head == nil {
		return nil, errors.New("could not retrieve block height")
	}

	currentHeight, err := head.(types.TipSet).Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(currentHeight), nil
}
