package api

import (
	"context"

	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// Block is the interface that defines methods to get human-readable
// represenations of Filecoin objects.
type Block interface {
	Get(ctx context.Context, id *cid.Cid) (*types.Block, error)
}
