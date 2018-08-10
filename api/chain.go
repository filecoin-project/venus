package api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

// Chain is the interface that defines methods to inspect the Filecoin blockchain.
type Chain interface {
	Head() ([]*cid.Cid, error)
	Ls(ctx context.Context) <-chan interface{}
}
