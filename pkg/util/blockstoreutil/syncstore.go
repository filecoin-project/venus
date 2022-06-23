package blockstoreutil

import (
	"context"
	"sync"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

var _ Blockstore = (*SyncStore)(nil)

type SyncStore struct {
	mu sync.RWMutex
	bs MemBlockstore // specifically use a memStore to save indirection overhead.
}

func (m *SyncStore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bs.View(ctx, cid, callback)
}

func (m *SyncStore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bs.DeleteMany(ctx, cids)
}
func (m *SyncStore) DeleteBlock(ctx context.Context, k cid.Cid) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bs.DeleteBlock(ctx, k)
}
func (m *SyncStore) Has(ctx context.Context, k cid.Cid) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bs.Has(ctx, k)
}
func (m *SyncStore) Get(ctx context.Context, k cid.Cid) (blocks.Block, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bs.Get(ctx, k)
}

// GetSize returns the CIDs mapped BlockSize
func (m *SyncStore) GetSize(ctx context.Context, k cid.Cid) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bs.GetSize(ctx, k)
}

// Put puts a given block to the underlying datastore
func (m *SyncStore) Put(ctx context.Context, b blocks.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bs.Put(ctx, b)
}

// PutMany puts a slice of blocks at the same time using batching
// capabilities of the underlying datastore whenever possible.
func (m *SyncStore) PutMany(ctx context.Context, bs []blocks.Block) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bs.PutMany(ctx, bs)
}

// AllKeysChan returns a channel from which
// the CIDs in the blockstore can be read. It should respect
// the given context, closing the channel if it becomes Done.
func (m *SyncStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// this blockstore implementation doesn't do any async work.
	return m.bs.AllKeysChan(ctx)
}

// HashOnRead specifies if every read block should be
// rehashed to make sure it matches its CID.
func (m *SyncStore) HashOnRead(enabled bool) {
	// noop
}
