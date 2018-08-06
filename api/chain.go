package api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

type ChainAPI interface {
	Head() ([]*cid.Cid, error)
	Ls(ctx context.Context) <-chan interface{}
}
