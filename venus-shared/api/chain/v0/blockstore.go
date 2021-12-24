package v0

import (
	"context"

	chain2 "github.com/filecoin-project/venus/venus-shared/api/chain"

	"github.com/ipfs/go-cid"
)

type IBlockStore interface {
	ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error)                       //perm:read
	ChainDeleteObj(ctx context.Context, obj cid.Cid) error                               //perm:admin
	ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error)                          //perm:read
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (chain2.ObjStat, error) //perm:read
}
