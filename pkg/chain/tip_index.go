package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

var (
	// ErrNotFound is returned when the key for a "Get" lookup is not in the index.
	ErrNotFound = errors.New("Key not found in tipindex")
)

// TipSetMetadata is the type stored at the leaves of the TipStateCache.  It contains
// a tipset pointing to blocks, the root cid of the chain's state after
// applying the messages in this tipset to it's parent state, and the cid of the receipts
// for these messages.
type TipSetMetadata struct {
	// TipSetStateRoot is the root of aggregate state after applying tipset
	TipSetStateRoot cid.Cid

	// TipSet is the set of blocks that forms the tip set
	TipSet *types.TipSet

	// TipSetReceipts receipts from all message contained within this tipset
	TipSetReceipts cid.Cid
}

type tipLoader interface {
	GetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error)
	LoadTipsetMetadata(ctx context.Context, ts *types.TipSet) (*TipSetMetadata, error)
}

// TipStateCache tracks tipsets and their states by tipset block ids.
// All methods are thread safe.
type TipStateCache struct {
	// cache allows lookup of recorded TipSet states by TipSet ID.
	cache map[string]TSState

	loader tipLoader

	l sync.RWMutex
}

// NewTipStateCache is the TipStateCache constructor.
func NewTipStateCache(loader tipLoader) *TipStateCache {
	return &TipStateCache{
		cache:  make(map[string]TSState, 2880), // one day height
		loader: loader,
	}
}

// Put adds an entry to both of TipStateCache's internal indexes.
// After this call the input TipSetMetadata can be looked up by the ID of the tipset.
func (ti *TipStateCache) Put(tsm *TipSetMetadata) {
	ti.l.Lock()
	defer ti.l.Unlock()

	ti.cache[tsm.TipSet.String()] = TSState{
		StateRoot: tsm.TipSetStateRoot,
		Receipts:  tsm.TipSetReceipts,
	}
}

// Get returns the tipset given by the input ID and its state.
func (ti *TipStateCache) Get(ctx context.Context, ts *types.TipSet) (TSState, error) {
	ti.l.RLock()
	state, ok := ti.cache[ts.String()]
	ti.l.RUnlock()
	if !ok {
		tipSetMetadata, err := ti.loader.LoadTipsetMetadata(ctx, ts)
		if err != nil {
			return TSState{}, errors.New("state not exit")
		}
		ti.Put(tipSetMetadata)

		return TSState{
			StateRoot: tipSetMetadata.TipSetStateRoot,
			Receipts:  tipSetMetadata.TipSetReceipts,
		}, nil
	}
	return state, nil
}

// GetTipSetStateRoot returns the tipsetStateRoot from func (ti *TipStateCache) Get(tsKey string).
func (ti *TipStateCache) GetTipSetStateRoot(ctx context.Context, ts *types.TipSet) (cid.Cid, error) {
	state, err := ti.Get(ctx, ts)
	if err != nil {
		return cid.Cid{}, err
	}
	return state.StateRoot, nil
}

// GetTipSetReceiptsRoot returns the tipsetReceipts from func (ti *TipStateCache) Get(tsKey string).
func (ti *TipStateCache) GetTipSetReceiptsRoot(ctx context.Context, ts *types.TipSet) (cid.Cid, error) {
	state, err := ti.Get(ctx, ts)
	if err != nil {
		return cid.Cid{}, err
	}
	return state.Receipts, nil
}

// Has returns true iff the tipset with the input ID is stored in
// the TipStateCache.
func (ti *TipStateCache) Has(ctx context.Context, ts *types.TipSet) bool {
	_, err := ti.Get(ctx, ts)
	return err == nil
}

func (ti *TipStateCache) Del(ts *types.TipSet) {
	ti.l.Lock()
	defer ti.l.Unlock()
	delete(ti.cache, ts.String())
}

// makeKey returns a unique string for every parent set key and height input
func makeKey(pKey string, h abi.ChainEpoch) string {
	return fmt.Sprintf("p-%s h-%d", pKey, h)
}
