package api

import (
	"context"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// Block is the interface that defines methods to get human-readable
// represenations of Filecoin objects.
type Block interface {
	Get(ctx context.Context, id cid.Cid) (*types.Block, error)
}
