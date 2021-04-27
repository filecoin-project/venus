package chain

import (
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	tcache "github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
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
	GetTipSet(key types.TipSetKey) (*types.TipSet, error)
	LoadTipsetMetadata(ts *types.TipSet) (*TipSetMetadata, error)
}

// TipStateCache tracks tipsets and their states by tipset block ids and parent
// block ids.  All methods are threadsafe as shared data is guarded by a
// mutex.
type TipStateCache struct {
	mu sync.Mutex
	// tsasByParents allows lookup of all TipSetAndStates with the same parent IDs.
	tsasByParentsAndHeight *tcache.Cache
	// tsasByID allows lookup of recorded TipSetAndStates by TipSet ID.
	tsasByID *tcache.Cache

	loader tipLoader
}

const (
	timeExpire    = time.Hour * 24
	cleanInterval = time.Hour * 12
)

// NewTipStateCache is the TipStateCache constructor.
func NewTipStateCache(loader tipLoader) *TipStateCache {
	return &TipStateCache{
		tsasByParentsAndHeight: tcache.New(timeExpire, cleanInterval),
		tsasByID:               tcache.New(timeExpire, cleanInterval),
		loader:                 loader,
	}
}

// Put adds an entry to both of TipStateCache's internal indexes.
// After this call the input TipSetMetadata can be looked up by the ID of
// the tipset, or the tipset's parent.
func (ti *TipStateCache) Put(tsas *TipSetMetadata) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	return ti.put(tsas)
}

// Put adds an entry to both of TipStateCache's internal indexes.
// After this call the input TipSetMetadata can be looked up by the ID of
// the tipset, or the tipset's parent.
func (ti *TipStateCache) put(tsas *TipSetMetadata) error {
	tsKey := tsas.TipSet.String()
	// Update tsasByID
	ti.tsasByID.Set(tsKey, tsas, timeExpire)

	// Update tsasByParents
	pSet := tsas.TipSet.Parents()
	pKey := pSet.String()
	h := tsas.TipSet.Height()
	key := makeKey(pKey, h)
	tsasByID, ok := ti.tsasByParentsAndHeight.Get(key)
	if !ok {
		tsasByID = make(map[string]*TipSetMetadata)
		ti.tsasByParentsAndHeight.Set(key, tsasByID, timeExpire)
	}
	tsasByID.(map[string]*TipSetMetadata)[tsKey] = tsas
	return nil
}

// Get returns the tipset given by the input ID and its state.
func (ti *TipStateCache) Get(ts *types.TipSet) (*TipSetMetadata, error) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	tsas, ok := ti.tsasByID.Get(ts.String())
	if !ok {
		tipSetMetadata, err := ti.loader.LoadTipsetMetadata(ts)
		if err != nil {
			return nil, xerrors.New("state not exit")
		}

		err = ti.put(tipSetMetadata)
		if err != nil {
			return nil, xerrors.New("failed to update tipstate")
		}

		return tipSetMetadata, nil
	}
	return tsas.(*TipSetMetadata), nil
}

// GetTipSetStateRoot returns the tipsetStateRoot from func (ti *TipStateCache) Get(tsKey string).
func (ti *TipStateCache) GetTipSetStateRoot(ts *types.TipSet) (cid.Cid, error) {
	tsas, err := ti.Get(ts)
	if err != nil {
		return cid.Cid{}, err
	}
	return tsas.TipSetStateRoot, nil
}

// GetTipSetReceiptsRoot returns the tipsetReceipts from func (ti *TipStateCache) Get(tsKey string).
func (ti *TipStateCache) GetTipSetReceiptsRoot(ts *types.TipSet) (cid.Cid, error) {
	tsas, err := ti.Get(ts)
	if err != nil {
		return cid.Cid{}, err
	}
	return tsas.TipSetReceipts, nil
}

// Has returns true iff the tipset with the input ID is stored in
// the TipStateCache.
func (ti *TipStateCache) Has(ts *types.TipSet) bool {
	_, err := ti.Get(ts)
	return err == nil
}

// GetSiblingState returns the all tipsets and states stored in the TipStateCache
// such that the parent ID of these tipsets equals the input.
func (ti *TipStateCache) GetSiblingState(ts *types.TipSet) ([]*TipSetMetadata, error) {
	pTS, err := ti.loader.GetTipSet(ts.Parents())
	if err != nil {
		return nil, err
	}
	pKey := makeKey(pTS.Key().String(), ts.Height())
	ti.mu.Lock()
	defer ti.mu.Unlock()
	tsasByID, ok := ti.tsasByParentsAndHeight.Get(pKey)
	if !ok {
		return nil, ErrNotFound
	}
	var ret []*TipSetMetadata
	for _, tsas := range tsasByID.(map[string]*TipSetMetadata) {
		ret = append(ret, tsas)
	}
	return ret, nil
}

// HasSiblingState returns true iff there exist tipsets, and states,
// tracked in the TipStateCache such that the parent ID of these tipsets equals the
// input.
func (ti *TipStateCache) HasSiblingState(ts *types.TipSet) bool {
	pTS, err := ti.loader.GetTipSet(ts.Parents())
	if err != nil {
		return false
	}

	pKey := makeKey(pTS.Key().String(), ts.Height())
	ti.mu.Lock()
	defer ti.mu.Unlock()
	_, ok := ti.tsasByParentsAndHeight.Get(pKey)
	return ok
}

// makeKey returns a unique string for every parent set key and height input
func makeKey(pKey string, h abi.ChainEpoch) string {
	return fmt.Sprintf("p-%s h-%d", pKey, h)
}
