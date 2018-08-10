package api

import (
	"context"

	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"

	"github.com/filecoin-project/go-filecoin/types"
)

// Block is the interface that defines methods to get human-readable
// represenations of Filecoin objects.
type Block interface {
	Get(ctx context.Context, id *cid.Cid) (*types.Block, error)
}
