package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	blockadt "github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/types"
	lru "github.com/hashicorp/golang-lru"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/cborutil"
	"github.com/filecoin-project/venus/pkg/enccid"
	"github.com/filecoin-project/venus/pkg/encoding"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

// HeadChangeTopic is the topic used to publish new heads.
const (
	HeadChangeTopic = "headchange"
	HCRevert        = "revert"
	HCApply         = "apply"
	HCCurrent       = "current"
)

// GenesisKey is the key at which the genesis Cid is written in the datastore.
var GenesisKey = datastore.NewKey("/consensus/genesisCid")

var logStore = logging.Logger("chain.store")

// HeadKey is the key at which the head tipset cid's are written in the datastore.
var HeadKey = datastore.NewKey("/chain/heaviestTipSet")

var ErrNotifeeDone = errors.New("notifee is done and should be removed")

// ReorgNotifee represents a callback that gets called upon reorgs.
type ReorgNotifee func(rev, app []*block.TipSet) error

type reorg struct {
	old []*block.TipSet
	new []*block.TipSet
}

type HeadChange struct {
	Type string
	Val  *block.TipSet
}

// CheckPoint is the key which the check-point written in the datastore.
var CheckPoint = datastore.NewKey("/chain/checkPoint")

type ipldSource struct {
	// cst is a store allowing access
	// (un)marshalling and interop with go-ipld-hamt.
	cborStore cbor.IpldStore
}

type tsState struct {
	StateRoot enccid.Cid
	Reciepts  enccid.Cid
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

	bsstore blockstore.Blockstore

	localbs blockstore.Blockstore

	localviewer blockstore.Viewer

	heaviestLk sync.Mutex
	heaviest   *block.TipSet

	mmCache *lru.ARCCache

	// ds is the datastore for the chain's private metadata which consists
	// of the tipset key to state root cid mapping, and the heaviest tipset
	// key.
	ds repo.Datastore

	// genesis is the CID of the genesis block.
	genesis cid.Cid
	// head is the tipset at the head of the best known chain.
	head *block.TipSet

	checkPoint block.TipSetKey
	// Protects head and genesisCid.
	mu sync.RWMutex

	// headEvents is a pubsub channel that publishes an event every time the head changes.
	// We operate under the assumption that tipsets published to this channel
	// will always be queued and delivered to subscribers in the order discovered.
	// Successive published tipsets may be supersets of previously published tipsets.
	// TODO: rename to notifications.  Also, reconsider ordering assumption depending
	// on decisions made around the FC node notification system.
	// TODO: replace this with a synchronous event bus
	// https://github.com/filecoin-project/venus/issues/2309
	headEvents *pubsub.PubSub

	// Tracks tipsets by height/parentset for use by expected consensus.
	tipIndex *TipIndex

	// Reporter is used by the store to update the current status of the chain.
	reporter Reporter

	chainIndex *ChainIndex

	notifees []ReorgNotifee

	reorgCh chan reorg
}

// NewStore constructs a new default store.
func NewStore(ds repo.Datastore,
	cst cbor.IpldStore,
	bsstore blockstore.Blockstore,
	sr Reporter,
	genesisCid cid.Cid,
) *Store {
	ipldSource := newSource(cst)
	tipsetProvider := TipSetProviderFromBlocks(context.TODO(), ipldSource)
	store := &Store{
		stateAndBlockSource: ipldSource,
		ds:                  ds,
		bsstore:             bsstore,
		headEvents:          pubsub.New(64),
		tipIndex:            NewTipIndex(),
		checkPoint:          block.UndefTipSet.Key(),
		genesis:             genesisCid,
		reporter:            sr,
		chainIndex:          NewChainIndex(tipsetProvider.GetTipSet),
		notifees:            []ReorgNotifee{},
	}

	val, err := store.ds.Get(CheckPoint)
	if err != nil {
		store.checkPoint = block.NewTipSetKey(genesisCid)
	} else {
		err = encoding.Decode(val, &store.checkPoint)
	}
	logStore.Infof("check point value: %v, error: %v", store.checkPoint, err)

	store.reorgCh = store.reorgWorker(context.TODO())
	return store
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

	var checkPointTs *block.TipSet
	loopBack := abi.ChainEpoch(0)
	if !store.checkPoint.Empty() {
		checkPointTs, err = LoadTipSetBlocks(ctx, store.stateAndBlockSource, store.checkPoint)
		if err != nil {
			return errors.Wrap(err, "error loading head tipset")
		}
		loopBack = checkPointTs.EnsureHeight() - 10
	}

	startHeight := headTs.At(0).Height
	logStore.Infof("start loading chain at tipset: %s, height: %d", headTsKey.String(), startHeight)
	// Ensure we only produce 10 log messages regardless of the chain height.
	logStatusEvery := startHeight / 10

	var startPoint *block.TipSet

	// Provide tipsets directly from the block store, not from the tipset index which is
	// being rebuilt by this traversal.
	tipsetProvider := TipSetProviderFromBlocks(ctx, store.stateAndBlockSource)
	for iterator := IterAncestors(ctx, tipsetProvider, headTs); !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return err
		}
		startPoint = iterator.Value()

		height, err := startPoint.Height()
		if err != nil {
			return err
		}
		if logStatusEvery != 0 && (height%logStatusEvery) == 0 {
			logStore.Infof("load tipset: %s, height: %v", startPoint.String(), height)
		}

		stateRoot, receipts, err := store.loadStateRootAndReceipts(startPoint)
		if err != nil {
			return err
		}

		err = store.PutTipSetMetadata(ctx, &TipSetMetadata{
			TipSet:          startPoint,
			TipSetStateRoot: stateRoot,
			TipSetReceipts:  receipts,
		})
		if err != nil {
			return err
		}

		if startPoint.EnsureHeight() <= loopBack {
			break
		}
	}

	logStore.Infof("finished loading %d tipsets from %s", startHeight, headTs.String())

	//todo just for test should remove if ok, 新创建节点会出问题?
	/*	if checkPointTs == nil || headTs.EnsureHeight() > checkPointTs.EnsureHeight() {
		p, err := headTs.Parents()
		if err != nil {
			return err
		}
		headTs, err = store.GetTipSet(p)
		if err != nil {
			return err
		}
	}*/

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

func (store *Store) loadStateRootAndReceipts(ts *block.TipSet) (cid.Cid, cid.Cid, error) {
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

	return metadata.StateRoot.Cid, metadata.Reciepts.Cid, nil
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

func (store *Store) DelTipSetMetadata(ctx context.Context, ts *block.TipSet) error {
	err := store.tipIndex.Del(ts)
	if err != nil {
		return err
	}

	// Persist the state mapping.
	if err = store.deleteTipSetMetadata(ts); err != nil {
		return err
	}

	return nil
}

// GetTipSet returns the tipset identified by `key`.
func (store *Store) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	blks := []*block.Block{}

	for _, id := range key.ToSlice() {
		blk, err := store.stateAndBlockSource.GetBlock(context.TODO(), id)

		if err != nil {
			return nil, err
		}
		blks = append(blks, blk)
	}
	ts, err := block.NewTipSet(blks...)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func (store *Store) GetTipSetByHeight(ctx context.Context, ts *block.TipSet, h abi.ChainEpoch, prev bool) (*block.TipSet, error) {
	if ts == nil {
		ts = store.head
	}

	if h > ts.EnsureHeight() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if h == ts.EnsureHeight() {
		return ts, nil
	}

	lbts, err := store.chainIndex.GetTipSetByHeight(ctx, ts, h)
	if err != nil {
		return nil, err
	}

	if lbts.EnsureHeight() < h {
		log.Warnf("chain index returned the wrong tipset at height %d, using slow retrieval", h)
		lbts, err = store.chainIndex.GetTipsetByHeightWithoutCache(ts, h)
		if err != nil {
			return nil, err
		}
	}

	if lbts.EnsureHeight() == h || !prev {
		return lbts, nil
	}

	return store.GetTipSet(lbts.EnsureParents())
}

// GetTipSetState returns the aggregate state of the tipset identified by `key`.
func (store *Store) GetTipSetState(ctx context.Context, key block.TipSetKey) (state.Tree, error) {
	stateCid, err := store.tipIndex.GetTipSetStateRoot(key)
	if err != nil {
		return nil, err
	}
	return state.LoadState(ctx, store.stateAndBlockSource.cborStore, stateCid)
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

func (store *Store) GetLatestBeaconEntry(ts *block.TipSet) (*block.BeaconEntry, error) {
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
		cur = next
	}

	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return &block.BeaconEntry{
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
			return pts, nil
		}

		ts = pts
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

// SetHead sets the passed in tipset as the new head of this chain.
func (store *Store) SetHead(ctx context.Context, newTs *block.TipSet) error {
	logStore.Infof("SetHead %s", newTs.String())

	// Add logging to debug sporadic test failure.
	if !newTs.Defined() {
		logStore.Errorf("publishing empty tipset")
		logStore.Error(debug.Stack())
	}

	dropped, added, update, err := func() ([]*block.TipSet, []*block.TipSet, bool, error) {
		var dropped []*block.TipSet
		var added []*block.TipSet
		var err error
		store.mu.Lock()
		defer store.mu.Unlock()

		if store.head != nil {
			if store.head.Equals(newTs) {
				return nil, nil, false, nil
			}
			//reorg
			oldHead := store.head
			dropped, added, err = CollectTipsToCommonAncestor(ctx, store, oldHead, newTs)
			if err != nil {
				return nil, nil, false, err
			}
		} else {
			added = []*block.TipSet{newTs}
		}

		// Ensure consistency by storing this new head on disk.
		if errInner := store.writeHead(ctx, newTs.Key()); errInner != nil {
			return nil, nil, false, errors.Wrap(errInner, "failed to write new Head to datastore")
		}
		store.head = newTs
		return dropped, added, true, nil
	}()

	if err != nil {
		return err
	}

	if !update {
		return nil
	}

	h, err := newTs.Height()
	if err != nil {
		return err
	}
	store.reporter.UpdateStatus(validateHead(newTs.Key()), validateHeight(h))

	//do reorg
	store.reorgCh <- reorg{
		old: dropped,
		new: added,
	}
	return nil
}

func (store *Store) reorgWorker(ctx context.Context) chan reorg {
	headChangeNotifee := func(rev, app []*block.TipSet) error {
		notif := make([]*HeadChange, len(rev)+len(app))
		for i, apply := range rev {
			notif[i] = &HeadChange{
				Type: HCRevert,
				Val:  apply,
			}
		}

		for i, revert := range app {
			notif[i+len(rev)] = &HeadChange{
				Type: HCApply,
				Val:  revert,
			}
		}
		// Publish an event that we have a new head.
		store.headEvents.Pub(notif, HeadChangeTopic)
		return nil
	}

	out := make(chan reorg, 32)
	notifees := []ReorgNotifee{headChangeNotifee}

	go func() {
		defer log.Warn("reorgWorker quit")
		for {
			select {
			case r := <-out:
				var toremove map[int]struct{}
				for i, hcf := range notifees {
					err := hcf(r.old, r.new)

					switch err {
					case nil:

					case ErrNotifeeDone:
						if toremove == nil {
							toremove = make(map[int]struct{})
						}
						toremove[i] = struct{}{}

					default:
						log.Error("head change func errored (BAD): ", err)
					}
				}

				if len(toremove) > 0 {
					newNotifees := make([]ReorgNotifee, 0, len(notifees)-len(toremove))
					for i, hcf := range notifees {
						_, remove := toremove[i]
						if remove {
							continue
						}
						newNotifees = append(newNotifees, hcf)
					}
					notifees = newNotifees
				}

			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (store *Store) SubHeadChanges(ctx context.Context) chan []*HeadChange {
	out := make(chan []*HeadChange, 16)
	store.mu.RLock()
	head := store.head
	store.mu.RUnlock()
	out <- []*HeadChange{{
		Type: HCCurrent,
		Val:  head,
	}}

	subCh := store.headEvents.Sub(HeadChangeTopic)
	go func() {
		defer close(out)
		var unsubOnce sync.Once

		for {
			select {
			case val, ok := <-subCh:
				if !ok {
					log.Warn("chain head sub exit loop")
					return
				}
				if len(out) > 5 {
					log.Warnf("head change sub is slow, has %d buffered entries", len(out))
				}
				select {
				case out <- val.([]*HeadChange):
				case <-ctx.Done():
				}
			case <-ctx.Done():
				unsubOnce.Do(func() {
					go store.headEvents.Unsub(subCh)
				})
			}
		}
	}()
	return out
}

func (store *Store) SubscribeHeadChanges(f ReorgNotifee) {
	store.notifees = append(store.notifees, f)
}

// ReadOnlyStateStore provides a read-only IPLD store for access to chain state.
func (store *Store) ReadOnlyStateStore() cborutil.ReadOnlyIpldStore {
	return cborutil.ReadOnlyIpldStore{IpldStore: store.stateAndBlockSource.cborStore}
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
		StateRoot: enccid.NewCid(tsm.TipSetStateRoot),
		Reciepts:  enccid.NewCid(tsm.TipSetReceipts),
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

// deleteTipSetMetadata delete the state root id from the datastore for the tipset key.
func (store *Store) deleteTipSetMetadata(ts *block.TipSet) error {
	h, err := ts.Height()
	if err != nil {
		return err
	}

	key := datastore.NewKey(makeKey(ts.String(), h))
	return store.ds.Delete(key)
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
	return store.genesis
}

func (store *Store) GenesisRootCid() cid.Cid {
	genesis, _ := store.stateAndBlockSource.GetBlock(context.TODO(), store.GenesisCid())
	return genesis.ParentStateRoot.Cid
}

func (store *Store) Import(r io.Reader) (*block.TipSet, error) {
	header, err := car.LoadCar(store.bsstore, r)
	if err != nil {
		return nil, xerrors.Errorf("loadcar failed: %w", err)
	}

	root, err := store.GetTipSet(block.NewTipSetKey(header.Roots...))
	if err != nil {
		return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
	}

	parent := root.EnsureParents()

	log.Info("import height: ", root.EnsureHeight(), " root: ", root.At(0).ParentStateRoot.Cid, " parents: ", root.At(0).Parents)
	parentTipset, err := store.GetTipSet(parent)
	if err != nil {
		return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
	}
	err = store.PutTipSetMetadata(context.Background(), &TipSetMetadata{
		TipSetStateRoot: root.At(0).ParentStateRoot.Cid,
		TipSet:          parentTipset,
		TipSetReceipts:  root.At(0).ParentMessageReceipts.Cid,
	})
	if err != nil {
		return nil, err
	}
	loopBack := 900
	curTipset := parentTipset
	for i := 0; i < loopBack; i++ {
		curTipsetKey := curTipset.EnsureParents()
		curParentTipset, err := store.GetTipSet(curTipsetKey)
		if err != nil {
			return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
		}

		if curParentTipset.EnsureHeight() == 0 {
			break
		}

		//save fake root
		err = store.PutTipSetMetadata(context.Background(), &TipSetMetadata{
			TipSetStateRoot: curTipset.At(0).ParentStateRoot.Cid,
			TipSet:          curParentTipset,
			TipSetReceipts:  curTipset.At(0).ParentMessageReceipts.Cid,
		})
		if err != nil {
			return nil, err
		}
		curTipset = curParentTipset
	}
	return parentTipset, nil
}

func (store *Store) SetCheckPoint(checkPoint block.TipSetKey) {
	store.checkPoint = checkPoint
}

// WriteCheckPoint writes the given cids to disk.
func (store *Store) WriteCheckPoint(ctx context.Context, cids block.TipSetKey) error {
	logStore.Infof("WriteCheckPoint %v", cids)
	val, err := encoding.Encode(cids)
	if err != nil {
		return err
	}

	return store.ds.Put(CheckPoint, val)
}

// GetCheckPoint get the check point from store or disk.
func (store *Store) GetCheckPoint() block.TipSetKey {
	return store.checkPoint
}

// Stop stops all activities and cleans up.
func (store *Store) Stop() {
	store.headEvents.Shutdown()
}

func (store *Store) Store(ctx context.Context) adt.Store {
	return ActorStore(ctx, store.bsstore)
}

func ActorStore(ctx context.Context, bs blockstore.Blockstore) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(bs))
}

//
//func (cs *Store) GetChainRandomness(ctx context.Context, blks []cid.Cid, pers crypto.DomainSeparationTag, round abi.ChainEpoch, entropy []byte) ([]byte, error) {
//	_, span := trace.StartSpan(ctx, "store.GetChainRandomness")
//	defer span.End()
//	span.AddAttributes(trace.Int64Attribute("round", int64(round)))
//
//	ts, err := cs.LoadTipSet(block.NewTipSetKey(blks...))
//	if err != nil {
//		return nil, err
//	}
//
//	if round > ts.Height() {
//		return nil, xerrors.Errorf("cannot draw randomness from the future")
//	}
//
//	searchHeight := round
//	if searchHeight < 0 {
//		searchHeight = 0
//	}
//
//	randTs, err := cs.GetTipsetByHeight(ctx, searchHeight, ts, true)
//	if err != nil {
//		return nil, err
//	}
//
//	mtb := randTs.MinTicketBlock()
//
//	// if at (or just past -- for null epochs) appropriate epoch
//	// or at genesis (works for negative epochs)
//	return DrawRandomness(mtb.Ticket.VRFProof, pers, round, entropy)
//}
//
//
//func (cs *Store) LoadTipSet(tsk block.TipSetKey) (*block.TipSet, error) {
//	v, err := cs.GetTipSet(tsk)
//	if err == nil {
//		return v, nil
//	}
//
//	// Fetch tipset block headers from blockstore in parallel
//	var eg errgroup.Group
//	cids := tsk.ToSlice()
//	blks := make([]*block.BlockHeader, len(cids))
//	for i, c := range cids {
//		i, c := i, c
//		eg.Go(func() error {
//			b, err := cs.bl(c)
//			if err != nil {
//				return xerrors.Errorf("get block %s: %w", c, err)
//			}
//
//			blks[i] = b
//			return nil
//		})
//	}
//	err := eg.Wait()
//	if err != nil {
//		return nil, err
//	}
//
//	ts, err := block.NewTipSet(blks)
//	if err != nil {
//		return nil, err
//	}
//
//	return ts, nil
//}

func (store *Store) GetCMessage(c cid.Cid) (types.ChainMsg, error) {
	m, err := store.GetMessage(c)
	if err == nil {
		return m, nil
	}
	if err != blockstore.ErrNotFound {
		log.Warnf("GetCMessage: unexpected error getting unsigned message: %s", err)
	}

	return store.GetSignedMessage(c)
}

func (store *Store) GetMessage(c cid.Cid) (*types.UnsignedMessage, error) {
	if store.localviewer == nil {
		sb, err := store.bsstore.Get(c)
		if err != nil {
			log.Errorf("get message get failed: %s: %s", c, err)
			return nil, err
		}
		return types.DecodeMessage(sb.RawData())
	}

	var msg *types.UnsignedMessage
	err := store.localviewer.View(c, func(b []byte) (err error) {
		msg, err = types.DecodeMessage(b)
		return err
	})
	return msg, err
}

func (store *Store) GetSignedMessage(c cid.Cid) (*types.SignedMessage, error) {
	if store.localviewer == nil {
		sb, err := store.bsstore.Get(c)
		if err != nil {
			log.Errorf("get message get failed: %s: %s", c, err)
			return nil, err
		}
		return types.DecodeSignedMessage(sb.RawData())
	}

	var msg *types.SignedMessage
	err := store.localviewer.View(c, func(b []byte) (err error) {
		msg, err = types.DecodeSignedMessage(b)
		return err
	})
	return msg, err
}

func (store *Store) GetHeaviestTipSet() *block.TipSet {
	store.heaviestLk.Lock()
	defer store.heaviestLk.Unlock()
	return store.heaviest
}

type BlockMessages struct {
	Miner         address.Address
	BlsMessages   []types.ChainMsg
	SecpkMessages []types.ChainMsg
	WinCount      int64
}

func (store *Store) MessagesForTipset(ts *block.TipSet) ([]types.ChainMsg, error) {
	bmsgs, err := store.BlockMsgsForTipset(ts)
	if err != nil {
		return nil, err
	}

	var out []types.ChainMsg
	for _, bm := range bmsgs {
		for _, blsm := range bm.BlsMessages {
			out = append(out, blsm)
		}

		for _, secm := range bm.SecpkMessages {
			out = append(out, secm)
		}
	}

	return out, nil
}

func (store *Store) BlockMsgsForTipset(ts *block.TipSet) ([]BlockMessages, error) {
	applied := make(map[address.Address]uint64)

	selectMsg := func(m *types.UnsignedMessage) (bool, error) {
		// The first match for a sender is guaranteed to have correct nonce -- the block isn't valid otherwise
		if _, ok := applied[m.From]; !ok {
			applied[m.From] = m.Nonce
		}

		if applied[m.From] != m.Nonce {
			return false, nil
		}

		applied[m.From]++

		return true, nil
	}

	var out []BlockMessages
	for _, b := range ts.Blocks() {

		bms, sms, err := store.MessagesForBlock(b)
		if err != nil {
			return nil, xerrors.Errorf("failed to get messages for block: %w", err)
		}

		bm := BlockMessages{
			Miner:         b.Miner,
			BlsMessages:   make([]types.ChainMsg, 0, len(bms)),
			SecpkMessages: make([]types.ChainMsg, 0, len(sms)),
			WinCount:      b.ElectionProof.WinCount,
		}

		for _, bmsg := range bms {
			b, err := selectMsg(bmsg.VMMessage())
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}

			if b {
				bm.BlsMessages = append(bm.BlsMessages, bmsg)
			}
		}

		for _, smsg := range sms {
			b, err := selectMsg(smsg.VMMessage())
			if err != nil {
				return nil, xerrors.Errorf("failed to decide whether to select message for block: %w", err)
			}

			if b {
				bm.SecpkMessages = append(bm.SecpkMessages, smsg)
			}
		}

		out = append(out, bm)
	}

	return out, nil
}

func (store *Store) MessagesForBlock(b *block.Block) ([]*types.UnsignedMessage, []*types.SignedMessage, error) {
	blscids, secpkcids, err := store.ReadMsgMetaCids(b.Cid())
	if err != nil {
		return nil, nil, err
	}

	blsmsgs, err := store.LoadMessagesFromCids(blscids)
	if err != nil {
		return nil, nil, xerrors.Errorf("loading bls messages for block: %w", err)
	}

	secpkmsgs, err := store.LoadSignedMessagesFromCids(secpkcids)
	if err != nil {
		return nil, nil, xerrors.Errorf("loading secpk messages for block: %w", err)
	}

	return blsmsgs, secpkmsgs, nil
}

func (store *Store) LoadMessagesFromCids(cids []cid.Cid) ([]*types.UnsignedMessage, error) {
	msgs := make([]*types.UnsignedMessage, 0, len(cids))
	for i, c := range cids {
		m, err := store.GetMessage(c)
		if err != nil {
			return nil, xerrors.Errorf("failed to get message: (%s):%d: %w", c, i, err)
		}

		msgs = append(msgs, m)
	}

	return msgs, nil
}

func (store *Store) LoadSignedMessagesFromCids(cids []cid.Cid) ([]*types.SignedMessage, error) {
	msgs := make([]*types.SignedMessage, 0, len(cids))
	for i, c := range cids {
		m, err := store.GetSignedMessage(c)
		if err != nil {
			return nil, xerrors.Errorf("failed to get message: (%s):%d: %w", c, i, err)
		}

		msgs = append(msgs, m)
	}

	return msgs, nil
}

type mmCids struct {
	bls   []cid.Cid
	secpk []cid.Cid
}

func (store *Store) ReadMsgMetaCids(mmc cid.Cid) ([]cid.Cid, []cid.Cid, error) {
	o, ok := store.mmCache.Get(mmc)
	if ok {
		mmcids := o.(*mmCids)
		return mmcids.bls, mmcids.secpk, nil
	}

	cst := cbor.NewCborStore(store.localbs)
	var msgmeta block.MsgMeta
	if err := cst.Get(context.TODO(), mmc, &msgmeta); err != nil {
		return nil, nil, xerrors.Errorf("failed to load msgmeta (%s): %w", mmc, err)
	}

	blscids, err := store.readAMTCids(msgmeta.BlsMessages)
	if err != nil {
		return nil, nil, xerrors.Errorf("loading bls message cids for block: %w", err)
	}

	secpkcids, err := store.readAMTCids(msgmeta.SecpkMessages)
	if err != nil {
		return nil, nil, xerrors.Errorf("loading secpk message cids for block: %w", err)
	}

	store.mmCache.Add(mmc, &mmCids{
		bls:   blscids,
		secpk: secpkcids,
	})

	return blscids, secpkcids, nil
}

func (store *Store) readAMTCids(root cid.Cid) ([]cid.Cid, error) {
	ctx := context.TODO()
	// block headers use adt0, for now.
	a, err := blockadt.AsArray(store.Store(ctx), root)
	if err != nil {
		return nil, xerrors.Errorf("amt load: %w", err)
	}

	var (
		cids    []cid.Cid
		cborCid cbg.CborCid
	)
	if err := a.ForEach(&cborCid, func(i int64) error {
		c := cid.Cid(cborCid)
		cids = append(cids, c)
		return nil
	}); err != nil {
		return nil, xerrors.Errorf("failed to traverse amt: %w", err)
	}

	if uint64(len(cids)) != a.Length() {
		return nil, xerrors.Errorf("found %d cids, expected %d", len(cids), a.Length())
	}

	return cids, nil
}
