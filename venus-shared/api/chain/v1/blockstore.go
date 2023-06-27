package v1

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type IBlockStore interface {
	ChainReadObj(ctx context.Context, cid cid.Cid) ([]byte, error)                      //perm:read
	ChainDeleteObj(ctx context.Context, obj cid.Cid) error                              //perm:admin
	ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error)                         //perm:read
	ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (types.ObjStat, error) //perm:read
	// ChainPutObj puts a given object into the block store
	ChainPutObj(context.Context, blocks.Block) error //perm:admin
}
