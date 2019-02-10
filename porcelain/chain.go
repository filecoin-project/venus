package porcelain

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-filecoin/consensus"
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

	currentHeight, err := head.(consensus.TipSet).Height()
	if err != nil {
		return nil, err
	}
	return types.NewBlockHeight(currentHeight), nil
}

// ChainBlocksSince returns all blocks up to and including the block height
func ChainBlocksSince(ctx context.Context, plumbing chPlumbing, since uint64) chan interface{} {
	out := make(chan interface{})
	lsCtx, cancelLs := context.WithCancel(ctx)

	go func() {
		defer close(out)
		for raw := range plumbing.ChainLs(lsCtx) {
			switch v := raw.(type) {
			case error:
				out <- v
				return
			case consensus.TipSet:
				height, err := v.Height()
				if err != nil {
					out <- err
					cancelLs()
					return
				}

				out <- v

				if height == since {
					cancelLs()
					return
				}
			default:
				out <- v
			}
		}
	}()

	return out
}