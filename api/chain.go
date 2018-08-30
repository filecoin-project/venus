package api

import (
	"context"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

// Chain is the interface that defines methods to inspect the Filecoin blockchain.
type Chain interface {
	Head() ([]*cid.Cid, error)
	Ls(ctx context.Context) <-chan interface{}
}
