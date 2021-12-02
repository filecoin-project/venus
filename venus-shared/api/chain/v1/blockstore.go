package v1

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/app/submodule/apitypes"
)

type IBlockStore interface {
	// Rule[perm:read]
	ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error)
	// Rule[perm:read]
	ChainDeleteObj(ctx context.Context, obj cid.Cid) error
	// Rule[perm:read]
	ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error)
	// Rule[perm:read]
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (apitypes.ObjStat, error)
}
