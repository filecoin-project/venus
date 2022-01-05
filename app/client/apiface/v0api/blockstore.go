package v0api

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"
)

type IBlockStore interface {
	// Rule[perm:read]
	ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error)
	// Rule[perm:admin]
	ChainDeleteObj(ctx context.Context, obj cid.Cid) error
	// Rule[perm:read]
	ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error)
	// Rule[perm:read]
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (types.ObjStat, error)
}
