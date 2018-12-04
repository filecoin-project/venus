package api

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
)

// Chain is the interface that defines methods to inspect the Filecoin blockchain.
type Chain interface {
	Head() ([]*cid.Cid, error)
	Ls(ctx context.Context) <-chan interface{}
}
