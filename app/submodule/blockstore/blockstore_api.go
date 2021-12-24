package blockstore

import (
	"context"
	"sync"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"golang.org/x/xerrors"

	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
)

var _ v1api.IBlockStore = &blockstoreAPI{}

type blockstoreAPI struct { //nolint
	blockstore *BlockstoreSubmodule
}

func (blockstoreAPI *blockstoreAPI) ChainReadObj(ctx context.Context, ocid cid.Cid) ([]byte, error) {
	blk, err := blockstoreAPI.blockstore.Blockstore.Get(ocid)
	if err != nil {
		return nil, xerrors.Errorf("blockstore get: %w", err)
	}

	return blk.RawData(), nil
}

func (blockstoreAPI *blockstoreAPI) ChainDeleteObj(ctx context.Context, obj cid.Cid) error {
	return blockstoreAPI.blockstore.Blockstore.DeleteBlock(obj)
}

func (blockstoreAPI *blockstoreAPI) ChainHasObj(ctx context.Context, obj cid.Cid) (bool, error) {
	return blockstoreAPI.blockstore.Blockstore.Has(obj)
}

func (blockstoreAPI *blockstoreAPI) ChainStatObj(ctx context.Context, obj cid.Cid, base cid.Cid) (apitypes.ObjStat, error) {
	bs := blockstoreAPI.blockstore.Blockstore
	bsvc := blockservice.New(bs, offline.Exchange(bs))

	dag := merkledag.NewDAGService(bsvc)

	seen := cid.NewSet()

	var statslk sync.Mutex
	var stats apitypes.ObjStat
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
			return apitypes.ObjStat{}, err
		}
		collect = true
	}

	if err := merkledag.Walk(ctx, walker, obj, seen.Visit, merkledag.Concurrent()); err != nil {
		return apitypes.ObjStat{}, err
	}

	return stats, nil
}
