package chain

import (
	"context"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"golang.org/x/xerrors"
	"os"
	"runtime/debug"
	"sync"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/internal/pkg/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// NewHeadTopic is the topic used to publish new heads.
const NewHeadTopic = "new-head"

// GenesisKey is the key at which the genesis Cid is written in the datastore.
var GenesisKey = datastore.NewKey("/consensus/genesisCid")

var logStore = logging.Logger("chain.store")

// HeadKey is the key at which the head tipset cid's are written in the datastore.
var HeadKey = datastore.NewKey("/chain/heaviestTipSet")

type ipldSource struct {
	// cst is a store allowing access
	// (un)marshalling and interop with go-ipld-hamt.
	cborStore cbor.IpldStore
}

type tsState struct {
	StateRoot cid.Cid
	Reciepts  cid.Cid
}

func newSource(cst cbor.IpldStore) *ipldSource {
	return &ipldSource{
		cborStore: cst,
	}
}

// GetBlock retrieves a filecoin block by cid from the IPLD store.
func (source *ipldSource) GetBlock(ctx context.Context, c cid.Cid) (*block.Block, error) {
	var block block.Block

	err := source.cborStore.Get(ctx, c, &block)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", c.String())
	}
	return &block, nil
}

// Store is a generic implementation of the Store interface.
// It works(tm) for now.
type Store struct {
	// ipldSource is a wrapper around ipld storage.  It is used
	// for reading filecoin block and state objects kept by the node.
	stateAndBlockSource *ipldSource

	// ds is the datastore for the chain's private metadata which consists
	// of the tipset key to state root cid mapping, and the heaviest tipset
	// key.
	ds repo.Datastore

	// genesis is the CID of the genesis block.
	genesis cid.Cid
	// head is the tipset at the head of the best known chain.
	head block.TipSet
	// Protects head and genesisCid.
	mu sync.RWMutex

	// headEvents is a pubsub channel that publishes an event every time the head changes.
	// We operate under the assumption that tipsets published to this channel
	// will always be queued and delivered to subscribers in the order discovered.
	// Successive published tipsets may be supersets of previously published tipsets.
	// TODO: rename to notifications.  Also, reconsider ordering assumption depending
	// on decisions made around the FC node notification system.
	// TODO: replace this with a synchronous event bus
	// https://github.com/filecoin-project/go-filecoin/issues/2309
	headEvents *pubsub.PubSub

	// Tracks tipsets by height/parentset for use by expected consensus.
	tipIndex *TipIndex

	// Reporter is used by the store to update the current status of the chain.
	reporter Reporter
}

// NewStore constructs a new default store.
func NewStore(ds repo.Datastore, cst cbor.IpldStore, sr Reporter, genesisCid cid.Cid) *Store {
	return &Store{
		stateAndBlockSource: newSource(cst),
		ds:                  ds,
		headEvents:          pubsub.New(12),
		tipIndex:            NewTipIndex(),
		genesis:             genesisCid,
		reporter:            sr,
	}
}

// Load rebuilds the Store's caches by traversing backwards from the
// most recent best head as stored in its datastore.  Because Load uses a
// content addressed datastore it guarantees that parent blocks are correctly
// resolved from the datastore.  Furthermore Load ensures that all tipsets
// references correctly have the same parent height, weight and parent set.
// However, Load DOES NOT validate state transitions, it assumes that the
// tipset were only Put to the Store after checking for valid transitions.
//
// Furthermore Load trusts that the Store's backing datastore correctly
// preserves the cids of the heaviest tipset under the "HeadKey" datastore key.
// If the HeadKey cids are tampered with and invalid blocks added to the datastore
// then Load could be tricked into loading an invalid chain. Load will error if the
// head does not link back to the expected genesis block, or the Store's
// datastore does not store a link in the chain.  In case of error the caller
// should not consider the chain useable and propagate the error.
func (store *Store) Load(ctx context.Context) (err error) {
	ctx, span := trace.StartSpan(ctx, "Store.Load")
	defer tracing.AddErrorEndSpan(ctx, span, &err)

	// Clear the tipset index.
	store.tipIndex = NewTipIndex()

	headTsKey, err := store.loadHead()
	if err != nil {
		return err
	}

	headTs, err := LoadTipSetBlocks(ctx, store.stateAndBlockSource, headTsKey)
	if err != nil {
		return errors.Wrap(err, "error loading head tipset")
	}
	startHeight := headTs.At(0).Height
	logStore.Infof("start loading chain at tipset: %s, height: %d", headTsKey.String(), startHeight)
	// Ensure we only produce 10 log messages regardless of the chain height.
	logStatusEvery := startHeight / 10

	var genesii block.TipSet
	// Provide tipsets directly from the block store, not from the tipset index which is
	// being rebuilt by this traversal.
	tipsetProvider := TipSetProviderFromBlocks(ctx, store.stateAndBlockSource)
	for iterator := IterAncestors(ctx, tipsetProvider, headTs); !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return err
		}

		height, err := iterator.Value().Height()
		if err != nil {
			return err
		}
		if logStatusEvery != 0 && (height%logStatusEvery) == 0 {
			logStore.Infof("load tipset: %s, height: %v", iterator.Value().String(), height)
		}
		stateRoot, receipts, err := store.loadStateRootAndReceipts(iterator.Value())
		if err != nil {
			return err
		}
		err = store.PutTipSetMetadata(ctx, &TipSetMetadata{
			TipSet:          iterator.Value(),
			TipSetStateRoot: stateRoot,
			TipSetReceipts:  receipts,
		})
		if err != nil {
			return err
		}

		genesii = iterator.Value()
	}
	// Check genesis here.
	if genesii.Len() != 1 {
		return errors.Errorf("load terminated with tipset of %d blocks, expected genesis with exactly 1", genesii.Len())
	}

	loadCid := genesii.At(0).Cid()
	if !loadCid.Equals(store.genesis) {
		return errors.Errorf("expected genesis cid: %s, loaded genesis cid: %s", store.genesis, loadCid)
	}

	logStore.Infof("finished loading %d tipsets from %s", startHeight, headTs.String())
	// Set actual head.
	return store.SetHead(ctx, headTs)
}

// loadHead loads the latest known head from disk.
func (store *Store) loadHead() (block.TipSetKey, error) {
	var emptyCidSet block.TipSetKey
	bb, err := store.ds.Get(HeadKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read HeadKey")
	}

	var cids block.TipSetKey
	err = encoding.Decode(bb, &cids)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to cast headCids")
	}

	return cids, nil
}

func (store *Store) loadStateRootAndReceipts(ts block.TipSet) (cid.Cid, cid.Cid, error) {
	h, err := ts.Height()
	if err != nil {
		return cid.Undef, cid.Undef, err
	}
	key := datastore.NewKey(makeKey(ts.String(), h))
	bb, err := store.ds.Get(key)
	if err != nil {
		return cid.Undef, cid.Undef, errors.Wrapf(err, "failed to read tipset key %s", ts.String())
	}

	var metadata tsState
	err = encoding.Decode(bb, &metadata)
	if err != nil {
		return cid.Undef, cid.Undef, errors.Wrapf(err, "failed to decode tip set metadata %s", ts.String())
	}

	return metadata.StateRoot, metadata.Reciepts, nil
}

// PutTipSetMetadata persists the blocks of a tipset and the tipset index.
func (store *Store) PutTipSetMetadata(ctx context.Context, tsm *TipSetMetadata) error {
	// Update tipindex.
	err := store.tipIndex.Put(tsm)
	if err != nil {
		return err
	}
	// Persist the state mapping.
	if err = store.writeTipSetMetadata(tsm); err != nil {
		return err
	}

	return nil
}

// GetTipSet returns the tipset identified by `key`.
func (store *Store) GetTipSet(key block.TipSetKey) (block.TipSet, error) {
	return store.tipIndex.GetTipSet(key)
}

// GetTipSetState returns the aggregate state of the tipset identified by `key`.
func (store *Store) GetTipSetState(ctx context.Context, key block.TipSetKey) (state.Tree, error) {
	stateCid, err := store.tipIndex.GetTipSetStateRoot(key)
	if err != nil {
		return nil, err
	}
	return state.LoadState(ctx, store.stateAndBlockSource.cborStore, stateCid)
}

// GetGenesisState returns the state tree at genesis to retrieve initialization parameters.
func (store *Store) GetGenesisState(ctx context.Context) (state.Tree, error) {
	// retrieve genesis block
	genesis, err := store.stateAndBlockSource.GetBlock(ctx, store.GenesisCid())
	if err != nil {
		return nil, err
	}

	// create state tree
	return state.LoadState(ctx, store.stateAndBlockSource.cborStore, genesis.StateRoot)
}

// GetGenesisBlock returns the genesis block held by the chain store.
func (store *Store) GetGenesisBlock(ctx context.Context) (*block.Block, error) {
	return store.stateAndBlockSource.GetBlock(ctx, store.GenesisCid())
}

// GetTipSetStateRoot returns the aggregate state root CID of the tipset identified by `key`.
func (store *Store) GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error) {
	return store.tipIndex.GetTipSetStateRoot(key)
}

// GetTipSetReceiptsRoot returns the root CID of the message receipts for the tipset identified by `key`.
func (store *Store) GetTipSetReceiptsRoot(key block.TipSetKey) (cid.Cid, error) {
	return store.tipIndex.GetTipSetReceiptsRoot(key)
}

// HasTipSetAndState returns true iff the default store's tipindex is indexing
// the tipset identified by `key`.
func (store *Store) HasTipSetAndState(ctx context.Context, key block.TipSetKey) bool {
	return store.tipIndex.Has(key)
}

// GetTipSetAndStatesByParentsAndHeight returns the the tipsets and states tracked by
// the default store's tipIndex that have parents identified by `parentKey`.
func (store *Store) GetTipSetByHeight(parentKey block.TipSetKey, h abi.ChainEpoch, prev bool) (*block.TipSet, error) {
	pts, err := store.GetTipSet(parentKey)
	if err != nil {
		return nil, err
	}
	if h > pts.EnsureHeight() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if h == pts.EnsureHeight() {
		return &pts, nil
	}
	lbts, err := store.walkBack(&pts, h)
	if err != nil {
		return nil, err
	}
	if lbts.EnsureHeight() == h || !prev {
		return lbts, nil
	}

	pts, err = store.GetTipSet(lbts.EnsureParents())
	if err != nil {
		return nil, err
	}
	return &pts, nil
}

func (store *Store) GetLatestBeaconEntry(ts *block.TipSet) (*drand.Entry, error) {
	cur := ts
	for i := 0; i < 20; i++ {
		cbe := cur.At(0).BeaconEntries
		if len(cbe) > 0 {
			return cbe[len(cbe)-1], nil
		}

		if cur.EnsureHeight() == 0 {
			return nil, xerrors.Errorf("made it back to genesis block without finding beacon entry")
		}

		next, err := store.GetTipSet(cur.EnsureParents())
		if err != nil {
			return nil, xerrors.Errorf("failed to load parents when searching back for latest beacon entry: %w", err)
		}
		cur = &next
	}

	if os.Getenv("LOTUS_IGNORE_DRAND") == "_yes_" {
		return &drand.Entry{
			Data: []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
		}, nil
	}

	return nil, xerrors.Errorf("found NO beacon entries in the 20 blocks prior to given tipset")
}

func (store *Store) walkBack(from *block.TipSet, to abi.ChainEpoch) (*block.TipSet, error) {
	if to > from.EnsureHeight() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if to == from.EnsureHeight() {
		return from, nil
	}

	ts := from

	for {
		pts, err := store.GetTipSet(ts.EnsureParents())
		if err != nil {
			return nil, err
		}

		if to > pts.EnsureHeight() {
			// in case pts is lower than the epoch we're looking for (null blocks)
			// return a tipset above that height
			return ts, nil
		}
		if to == pts.EnsureHeight() {
			return &pts, nil
		}

		ts = &pts
	}
}

// GetTipSetAndStatesByParentsAndHeight returns the the tipsets and states tracked by
// the default store's tipIndex that have parents identified by `parentKey`.
func (store *Store) GetTipSetAndStatesByParentsAndHeight(parentKey block.TipSetKey, h abi.ChainEpoch) ([]*TipSetMetadata, error) {
	return store.tipIndex.GetByParentsAndHeight(parentKey, h)
}

// HasTipSetAndStatesWithParentsAndHeight returns true if the default store's tipindex
// contains any tipset identified by `parentKey`.
func (store *Store) HasTipSetAndStatesWithParentsAndHeight(parentKey block.TipSetKey, h abi.ChainEpoch) bool {
	return store.tipIndex.HasByParentsAndHeight(parentKey, h)
}

// HeadEvents returns a pubsub interface the pushes events each time the
// default store's head is reset.
func (store *Store) HeadEvents() *pubsub.PubSub {
	return store.headEvents
}

// SetHead sets the passed in tipset as the new head of this chain.
func (store *Store) SetHead(ctx context.Context, ts block.TipSet) error {
	logStore.Debugf("SetHead %s", ts.String())

	// Add logging to debug sporadic test failure.
	if !ts.Defined() {
		logStore.Errorf("publishing empty tipset")
		logStore.Error(debug.Stack())
	}

	noop, err := store.setHeadPersistent(ctx, ts)
	if err != nil {
		return err
	}
	if noop {
		// exit without sending head events if head was already set to ts
		return nil
	}

	h, err := ts.Height()
	if err != nil {
		return err
	}
	store.reporter.UpdateStatus(validateHead(ts.Key()), validateHeight(h))
	// Publish an event that we have a new head.
	store.HeadEvents().Pub(ts, NewHeadTopic)

	return nil
}

// ReadOnlyStateStore provides a read-only IPLD store for access to chain state.
func (store *Store) ReadOnlyStateStore() cborutil.ReadOnlyIpldStore {
	return cborutil.ReadOnlyIpldStore{IpldStore: store.stateAndBlockSource.cborStore}
}

func (store *Store) setHeadPersistent(ctx context.Context, ts block.TipSet) (bool, error) {
	// setHeaadPersistent sets the head in memory and on disk if the head is not
	// already set to ts.  If it is already set to ts it skips this and returns true
	store.mu.Lock()
	defer store.mu.Unlock()

	// Ensure consistency by storing this new head on disk.
	if errInner := store.writeHead(ctx, ts.Key()); errInner != nil {
		return false, errors.Wrap(errInner, "failed to write new Head to datastore")
	}
	if ts.Equals(store.head) {
		return true, nil
	}

	store.head = ts

	return false, nil
}

// writeHead writes the given cid set as head to disk.
func (store *Store) writeHead(ctx context.Context, cids block.TipSetKey) error {
	logStore.Debugf("WriteHead %s", cids.String())
	val, err := encoding.Encode(cids)
	if err != nil {
		return err
	}

	return store.ds.Put(HeadKey, val)
}

// writeTipSetMetadata writes the tipset key and the state root id to the
// datastore.
func (store *Store) writeTipSetMetadata(tsm *TipSetMetadata) error {
	if tsm.TipSetStateRoot == cid.Undef {
		return errors.New("attempting to write state root cid.Undef")
	}

	if tsm.TipSetReceipts == cid.Undef {
		return errors.New("attempting to write receipts cid.Undef")
	}

	metadata := tsState{
		StateRoot: tsm.TipSetStateRoot,
		Reciepts:  tsm.TipSetReceipts,
	}
	val, err := encoding.Encode(metadata)
	if err != nil {
		return err
	}

	// datastore keeps key:stateRoot (k,v) pairs.
	h, err := tsm.TipSet.Height()
	if err != nil {
		return err
	}
	key := datastore.NewKey(makeKey(tsm.TipSet.String(), h))
	return store.ds.Put(key, val)
}

// GetHead returns the current head tipset cids.
func (store *Store) GetHead() block.TipSetKey {
	store.mu.RLock()
	defer store.mu.RUnlock()

	if !store.head.Defined() {
		return block.TipSetKey{}
	}

	return store.head.Key()
}

// GenesisCid returns the genesis cid of the chain tracked by the default store.
func (store *Store) GenesisCid() cid.Cid {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.genesis
}

// Stop stops all activities and cleans up.
func (store *Store) Stop() {
	store.headEvents.Shutdown()
}
