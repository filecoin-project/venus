package chain

import (
	"context"
	"github.com/ipfs/go-cid"
)

type DbAPI struct {
	chain *ChainSubmodule
}

func NewDbAPI(chain *ChainSubmodule) DbAPI {
	return DbAPI{chain: chain}
}

func (dbAPI *DbAPI) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	return dbAPI.chain.State.ReadObj(ctx, ocid)
}

func (dbAPI *DbAPI) ChainHasObj(ctx context.Context, ocid cid.Cid) (bool, error) {
	return dbAPI.chain.State.HasObj(ctx, ocid)
}
