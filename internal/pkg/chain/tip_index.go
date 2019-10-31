package chain

import (
	"fmt"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
)

var (
	// ErrNotFound is returned when the key for a "Get" lookup is not in the index.
	ErrNotFound = errors.New("Key not found in tipindex")
)

// TipSetMetadata is the type stored at the leaves of the TipIndex.  It contains
// a tipset pointing to blocks, the root cid of the chain's state after
// applying the messages in this tipset to it's parent state, and the cid of the receipts
// for these messages.
type TipSetMetadata struct {
	// TipSetStateRoot is the root of aggregate state after applying tipset
	TipSetStateRoot cid.Cid

	// TipSet is the set of blocks that forms the tip set
	TipSet block.TipSet

	// TipSetReceipts receipts from all message contained within this tipset
	TipSetReceipts cid.Cid
}

type tsmByTipSetID map[string]*TipSetMetadata

// TipIndex tracks tipsets and their states by tipset block ids and parent
// block ids.  All methods are threadsafe as shared data is guarded by a
// mutex.
type TipIndex struct {
	mu sync.Mutex
	// tsasByParents allows lookup of all TipSetAndStates with the same parent IDs.
	tsasByParentsAndHeight map[string]tsmByTipSetID
	// tsasByID allows lookup of recorded TipSetAndStates by TipSet ID.
	tsasByID tsmByTipSetID
}

// NewTipIndex is the TipIndex constructor.
func NewTipIndex() *TipIndex {
	return &TipIndex{
		tsasByParentsAndHeight: make(map[string]tsmByTipSetID),
		tsasByID:               make(map[string]*TipSetMetadata),
	}
}

// Put adds an entry to both of TipIndex's internal indexes.
// After this call the input TipSetMetadata can be looked up by the ID of
// the tipset, or the tipset's parent.
func (ti *TipIndex) Put(tsas *TipSetMetadata) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	tsKey := tsas.TipSet.String()
	// Update tsasByID
	ti.tsasByID[tsKey] = tsas

	// Update tsasByParents
	pSet, err := tsas.TipSet.Parents()
	if err != nil {
		return err
	}
	pKey := pSet.String()
	h, err := tsas.TipSet.Height()
	if err != nil {
		return err
	}
	key := makeKey(pKey, h)
	tsasByID, ok := ti.tsasByParentsAndHeight[key]
	if !ok {
		tsasByID = make(map[string]*TipSetMetadata)
		ti.tsasByParentsAndHeight[key] = tsasByID
	}
	tsasByID[tsKey] = tsas
	return nil
}

// Get returns the tipset given by the input ID and its state.
func (ti *TipIndex) Get(tsKey block.TipSetKey) (*TipSetMetadata, error) {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	tsas, ok := ti.tsasByID[tsKey.String()]
	if !ok {
		return nil, ErrNotFound
	}
	return tsas, nil
}

// GetTipSet returns the tipset from func (ti *TipIndex) Get(tsKey string)
func (ti *TipIndex) GetTipSet(tsKey block.TipSetKey) (block.TipSet, error) {
	tsas, err := ti.Get(tsKey)
	if err != nil {
		return block.UndefTipSet, err
	}
	return tsas.TipSet, nil
}

// GetTipSetStateRoot returns the tipsetStateRoot from func (ti *TipIndex) Get(tsKey string).
func (ti *TipIndex) GetTipSetStateRoot(tsKey block.TipSetKey) (cid.Cid, error) {
	tsas, err := ti.Get(tsKey)
	if err != nil {
		return cid.Cid{}, err
	}
	return tsas.TipSetStateRoot, nil
}

// GetTipSetReceiptsRoot returns the tipsetReceipts from func (ti *TipIndex) Get(tsKey string).
func (ti *TipIndex) GetTipSetReceiptsRoot(tsKey block.TipSetKey) (cid.Cid, error) {
	tsas, err := ti.Get(tsKey)
	if err != nil {
		return cid.Cid{}, err
	}
	return tsas.TipSetReceipts, nil
}

// Has returns true iff the tipset with the input ID is stored in
// the TipIndex.
func (ti *TipIndex) Has(tsKey block.TipSetKey) bool {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	_, ok := ti.tsasByID[tsKey.String()]
	return ok
}

// GetByParentsAndHeight returns the all tipsets and states stored in the TipIndex
// such that the parent ID of these tipsets equals the input.
func (ti *TipIndex) GetByParentsAndHeight(pKey block.TipSetKey, h uint64) ([]*TipSetMetadata, error) {
	key := makeKey(pKey.String(), h)
	ti.mu.Lock()
	defer ti.mu.Unlock()
	tsasByID, ok := ti.tsasByParentsAndHeight[key]
	if !ok {
		return nil, ErrNotFound
	}
	var ret []*TipSetMetadata
	for _, tsas := range tsasByID {
		ret = append(ret, tsas)
	}
	return ret, nil
}

// HasByParentsAndHeight returns true iff there exist tipsets, and states,
// tracked in the TipIndex such that the parent ID of these tipsets equals the
// input.
func (ti *TipIndex) HasByParentsAndHeight(pKey block.TipSetKey, h uint64) bool {
	key := makeKey(pKey.String(), h)
	ti.mu.Lock()
	defer ti.mu.Unlock()
	_, ok := ti.tsasByParentsAndHeight[key]
	return ok
}

// makeKey returns a unique string for every parent set key and height input
func makeKey(pKey string, h uint64) string {
	return fmt.Sprintf("p-%s h-%d", pKey, h)
}
