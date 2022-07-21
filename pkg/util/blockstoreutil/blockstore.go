// blockstoreutil contains all the basic blockstore constructors used by lotus. Any
// blockstoreutil not ultimately constructed out of the building blocks in this
// package may not work properly.
//
//  * This package correctly wraps blockstores with the IdBlockstore. This blockstore:
//    * Filters out all puts for blocks with CIDs using the "identity" hash function.
//    * Extracts inlined blocks from CIDs using the identity hash function and
//      returns them on get/has, ignoring the contents of the blockstore.
//  * In the future, this package may enforce additional restrictions on block
//    sizes, CID validity, etc.
//
// To make auditing for misuse of blockstores tractable, this package re-exports
// parts of the go-ipfs-blockstore package such that no other package needs to
// import it directly.
package blockstoreutil

import (
	"context"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// NewTemporary returns a temporary blockstore.
func NewTemporary() MemBlockstore {
	return NewMemory()
}

// NewTemporarySync returns a thread-safe temporary blockstore.
func NewTemporarySync() *SyncStore {
	return &SyncStore{bs: NewMemory()}
}

// WrapIDStore wraps the underlying blockstore in an "identity" blockstore.
func WrapIDStore(bstore blockstore.Blockstore) Blockstore {
	return Adapt(blockstore.NewIdStore(bstore))
}

// NewBlockstore creates a new blockstore wrapped by the given datastore.
func NewBlockstore(dstore ds.Batching) Blockstore {
	return WrapIDStore(blockstore.NewBlockstore(dstore))
}

// Blockstore is the blockstore interface used by Lotus. It is the union
// of the basic go-ipfs blockstore, with other capabilities required by Lotus,
// e.g. View or Sync.
type Blockstore interface {
	blockstore.Blockstore
	blockstore.Viewer
	BatchDeleter
}

// Alias so other packages don't have to import go-ipfs-blockstore
//type Blockstore = blockstore.Blockstore
type Viewer = blockstore.Viewer
type GCBlockstore = blockstore.GCBlockstore
type CacheOpts = blockstore.CacheOpts
type GCLocker = blockstore.GCLocker

type BatchDeleter interface {
	DeleteMany(ctx context.Context, cids []cid.Cid) error
}

var NewGCLocker = blockstore.NewGCLocker
var NewGCBlockstore = blockstore.NewGCBlockstore

func DefaultCacheOpts() CacheOpts {
	return CacheOpts{
		HasBloomFilterSize:   0,
		HasBloomFilterHashes: 0,
		HasARCCacheSize:      512 << 10,
	}
}

func CachedBlockstore(ctx context.Context, bs blockstore.Blockstore, opts CacheOpts) (Blockstore, error) {
	bsTmp, err := blockstore.CachedBlockstore(ctx, bs, opts)
	if err != nil {
		return nil, err
	}
	return WrapIDStore(bsTmp), nil
}

type adaptedBlockstore struct {
	blockstore.Blockstore
}

var _ Blockstore = (*adaptedBlockstore)(nil)

func (a *adaptedBlockstore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	blk, err := a.Get(ctx, cid)
	if err != nil {
		return err
	}
	return callback(blk.RawData())
}

func (a *adaptedBlockstore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	for _, cid := range cids {
		err := a.DeleteBlock(ctx, cid)
		if err != nil {
			return err
		}
	}

	return nil
}

// Adapt adapts a standard blockstore to a Lotus blockstore by
// enriching it with the extra methods that Lotus requires (e.g. View, Sync).
//
// View proxies over to Get and calls the callback with the value supplied by Get.
// Sync noops.
func Adapt(bs blockstore.Blockstore) Blockstore {
	if ret, ok := bs.(Blockstore); ok {
		return ret
	}
	return &adaptedBlockstore{bs}
}
