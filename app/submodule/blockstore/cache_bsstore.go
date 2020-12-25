package blockstore

import (
	"context"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"time"
)

const CACHE_SIZE = 1000 * 10000 //nolint

var _ blockstore.Blockstore = (*CacheBlockStore)(nil)

func NewCacheBlockStore(bsstore blockstore.Blockstore, useLru bool) *CacheBlockStore {
	var cache blockstoreutil.IBlockCache
	if useLru {
		cache = blockstoreutil.NewLruCache(CACHE_SIZE)
	} else {
		cache = blockstoreutil.NewTimeCache(time.Hour*24, time.Hour*24)
	}
	return &CacheBlockStore{bsstore: bsstore, cache: cache}
}

type CacheBlockStore struct {
	bsstore blockstore.Blockstore
	cache   blockstoreutil.IBlockCache
}

func (c CacheBlockStore) DeleteBlock(bid cid.Cid) error {
	if _, ok := c.cache.Get(bid.String()); ok {
		c.cache.Remove(bid.String())
	}
	return c.bsstore.DeleteBlock(bid)
}

func (c CacheBlockStore) Has(bid cid.Cid) (bool, error) {
	if _, ok := c.cache.Get(bid.String()); ok {
		return true, nil
	}
	return c.bsstore.Has(bid)
}

func (c CacheBlockStore) Get(bid cid.Cid) (blocks.Block, error) {
	if val, ok := c.cache.Get(bid.String()); ok {
		return val.(blocks.Block), nil
	}

	val, err := c.bsstore.Get(bid)
	if err != nil {
		return nil, err
	}

	c.cache.Add(bid.String(), val)
	return val, nil
}

func (c CacheBlockStore) GetSize(bid cid.Cid) (int, error) {
	if val, ok := c.cache.Get(bid.String()); ok {
		return len(val.(blocks.Block).RawData()), nil
	}
	return c.bsstore.GetSize(bid)
}

func (c CacheBlockStore) Put(block blocks.Block) error {
	c.cache.Add(block.Cid().String(), block)
	return c.bsstore.Put(block)
}

func (c CacheBlockStore) PutMany(blocks []blocks.Block) error {
	for _, blk := range blocks {
		c.cache.Add(blk.Cid().String(), blk)
	}
	return c.bsstore.PutMany(blocks)
}

func (c CacheBlockStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return c.bsstore.AllKeysChan(ctx)
	/*ch, err := c.bsstore.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}
	output := make(chan cid.Cid, dsq.KeysOnlyBufSize)
	go func() {
		defer func() {
			close(output)
		}()
		var k cid.Cid
		var ok bool
		for {
			select {
			case k, ok = <-ch:
				if !ok {
					//complete
					return
				}
			case <-ctx.Done():
				return
			}

			select {
			case <-ctx.Done():
				return
			case output <- k:
			}
		}
	}()*/
}

func (c CacheBlockStore) HashOnRead(enabled bool) {
	c.bsstore.HashOnRead(enabled)
}
