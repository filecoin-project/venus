package blockstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/venus/venus-shared/types"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"

	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var _ v1api.IBlockStore = &blockstoreAPI{}

type blockstoreAPI struct { //nolint
	blockstore *BlockstoreSubmodule
}

func (blockstoreAPI *blockstoreAPI) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	blk, err := blockstoreAPI.blockstore.Blockstore.Get(ctx, ocid)
	if err != nil {
		return nil, fmt.Errorf("blockstore get: %w", err)
	}

	return blk.RawData(), nil
}

func (blockstoreAPI *blockstoreAPI) ChainDeleteObj(ctx context.Context, obj cid.Cid) error {
	return blockstoreAPI.blockstore.Blockstore.DeleteBlock(ctx, obj)
}

func (blockstoreAPI *blockstoreAPI) ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error) {
	return blockstoreAPI.blockstore.Blockstore.Has(ctx, obj)
}

func (blockstoreAPI *blockstoreAPI) ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (types.ObjStat, error) {
	bs := blockstoreAPI.blockstore.Blockstore
	bsvc := blockservice.New(bs, offline.Exchange(bs))

	dag := merkledag.NewDAGService(bsvc)

	seen := cid.NewSet()

	var statslk sync.Mutex
	var stats types.ObjStat
	var collect = true

	walker := func(ctx context.Context, c cid.Cid) ([]*ipld.Link, error) {
		if c.Prefix().Codec == cid.FilCommitmentSealed || c.Prefix().Codec == cid.FilCommitmentUnsealed {
			return []*ipld.Link{}, nil
		}

		nd, err := dag.Get(ctx, c)
		if err != nil {
			return nil, err
		}

		if collect {
			s := uint64(len(nd.RawData()))
			statslk.Lock()
			stats.Size = stats.Size + s
			stats.Links = stats.Links + 1
			statslk.Unlock()
		}

		return nd.Links(), nil
	}

	if base != cid.Undef {
		collect = false
		if err := merkledag.Walk(ctx, walker, base, seen.Visit, merkledag.Concurrent()); err != nil {
			return types.ObjStat{}, err
		}
		collect = true
	}

	if err := merkledag.Walk(ctx, walker, obj, seen.Visit, merkledag.Concurrent()); err != nil {
		return types.ObjStat{}, err
	}

	return stats, nil
}

func (blockstoreAPI *blockstoreAPI) ChainPutObj(ctx context.Context, blk blocks.Block) error {
	return blockstoreAPI.blockstore.Blockstore.Put(ctx, blk)
}

func (blockstoreAPI *blockstoreAPI) PutMany(ctx context.Context, blocks []blocks.Block) error {
	return blockstoreAPI.blockstore.Blockstore.PutMany(ctx, blocks)
}
