package chain

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime/debug"
	"sync"

	lru "github.com/hashicorp/golang-lru"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	_init "github.com/filecoin-project/venus/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/multisig"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/power"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/verifreg"
	"github.com/filecoin-project/venus/pkg/specactors/policy"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util"
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

var log = logging.Logger("chain.store")

// HeadKey is the key at which the head tipset cid's are written in the datastore.
var HeadKey = datastore.NewKey("/chain/heaviestTipSet")

var ErrNotifeeDone = errors.New("notifee is done and should be removed")

type loadTipSetFunc func(block.TipSetKey) (*block.TipSet, error)

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

type TsState struct {
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

	bsstore blockstore.Blockstore

	// ds is the datastore for the chain's private metadata which consists
	// of the tipset key to state root cid mapping, and the heaviest tipset
	// key.
	ds *CacheDs

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
	tipIndex *TipStateCache

	// Reporter is used by the store to update the current status of the chain.
	reporter Reporter

	circulatingSupplyCalculator *CirculatingSupplyCalculator

	chainIndex *ChainIndex

	reorgCh        chan reorg
	reorgNotifeeCh chan ReorgNotifee

	tsCache *lru.ARCCache
}

// NewStore constructs a new default store.
func NewStore(ds repo.Datastore,
	cst cbor.IpldStore,
	bsstore blockstore.Blockstore,
	sr Reporter,
	forkConfig *config.ForkUpgradeConfig,
	genesisCid cid.Cid,
) *Store {
	ipldSource := newSource(cst)
	cacheDs := NewCacheDs(ds, true)
	tsCache, _ := lru.NewARC(10000)
	store := &Store{
		stateAndBlockSource: ipldSource,
		ds:                  cacheDs,
		bsstore:             bsstore,
		headEvents:          pubsub.New(64),

		checkPoint:     block.UndefTipSet.Key(),
		genesis:        genesisCid,
		reporter:       sr,
		reorgNotifeeCh: make(chan ReorgNotifee),
		tsCache:        tsCache,
	}
	//todo cycle reference , may think a better idea
	store.tipIndex = NewTipStateCache(store)
	store.chainIndex = NewChainIndex(store.GetTipSet)
	store.circulatingSupplyCalculator = NewCirculatingSupplyCalculator(bsstore, store, forkConfig)

	val, err := store.ds.Get(CheckPoint)
	if err != nil {
		store.checkPoint = block.NewTipSetKey(genesisCid)
	} else {
		err = store.checkPoint.UnmarshalCBOR(bytes.NewReader(val))
	}
	log.Infof("check point value: %v, error: %v", store.checkPoint, err)

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

	headTsKey, err := store.loadHead()
	if err != nil {
		return err
	}

	headTs, err := LoadTipSetBlocks(ctx, store.stateAndBlockSource, headTsKey)
	if err != nil {
		return errors.Wrap(err, "error loading head tipset")
	}

	latestHeight := headTs.At(0).Height
	loopBack := latestHeight - policy.ChainFinality
	log.Infof("start loading chain at tipset: %s, height: %d", headTsKey.String(), latestHeight)

	// Provide tipsets directly from the block store, not from the tipset index which is
	// being rebuilt by this traversal.
	tipsetProvider := TipSetProviderFromBlocks(ctx, store.stateAndBlockSource)
	for iterator := IterAncestors(ctx, tipsetProvider, headTs); !iterator.Complete(); err = iterator.Next() {
		if err != nil {
			return err
		}
		ts := iterator.Value()

		tipSetMetadata, err := store.LoadTipsetMetadata(ts)
		if err != nil {
			return err
		}

		err = store.tipIndex.Put(tipSetMetadata)
		if err != nil {
			return err
		}

		if ts.EnsureHeight() <= loopBack {
			break
		}
	}

	log.Infof("finished loading %d tipsets from %s", latestHeight, headTs.String())

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
	tskBytes, err := store.ds.Get(HeadKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read HeadKey")
	}

	var tsk block.TipSetKey
	err = tsk.UnmarshalCBOR(bytes.NewReader(tskBytes))
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to cast headCids")
	}

	return tsk, nil
}

func (store *Store) LoadTipsetMetadata(ts *block.TipSet) (*TipSetMetadata, error) {
	h, err := ts.Height()
	if err != nil {
		return nil, err
	}
	key := datastore.NewKey(makeKey(ts.String(), h))
	tsStateBytes, err := store.ds.Get(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read tipset key %s", ts.String())
	}

	var metadata TsState
	err = metadata.UnmarshalCBOR(bytes.NewReader(tsStateBytes))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode tip set metadata %s", ts.String())
	}
	return &TipSetMetadata{
		TipSet:          ts,
		TipSetStateRoot: metadata.StateRoot,
		TipSetReceipts:  metadata.Reciepts,
	}, nil
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

// GetBlock returns the block identified by `cid`.
func (store *Store) GetBlock(blockID cid.Cid) (*block.Block, error) {
	return store.stateAndBlockSource.GetBlock(context.TODO(), blockID)
}

// GetTipSet returns the tipset identified by `key`.
func (store *Store) GetTipSet(key block.TipSetKey) (*block.TipSet, error) {
	if key.IsEmpty() {
		return store.GetHead(), nil
	}
	var blks []*block.Block
	val, has := store.tsCache.Get(key)
	if has {
		return val.(*block.TipSet), nil
	}

	for _, id := range key.Cids() {
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
	store.tsCache.Add(key, ts)
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
func (store *Store) GetTipSetState(ctx context.Context, ts *block.TipSet) (state.Tree, error) {
	stateCid, err := store.tipIndex.GetTipSetStateRoot(ts)
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
func (store *Store) GetTipSetStateRoot(key *block.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetStateRoot(key)
}

// GetTipSetReceiptsRoot returns the root CID of the message receipts for the tipset identified by `key`.
func (store *Store) GetTipSetReceiptsRoot(key *block.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetReceiptsRoot(key)
}

// HasTipSetAndState returns true iff the default store's tipindex is indexing
// the tipset identified by `key`.
func (store *Store) HasTipSetAndState(ctx context.Context, ts *block.TipSet) bool {
	return store.tipIndex.Has(ts)
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

// GetSiblingState returns the the tipsets and states tracked by
// the default store's tipIndex that have parents identified by `parentKey`.
func (store *Store) GetSiblingState(ts *block.TipSet) ([]*TipSetMetadata, error) {
	return store.tipIndex.GetSiblingState(ts)
}

// HasSiblingState returns true if the default store's tipindex
// contains any tipset identified by `parentKey`.
func (store *Store) HasSiblingState(ts *block.TipSet) bool {
	return store.tipIndex.HasSiblingState(ts)
}

// SetHead sets the passed in tipset as the new head of this chain.
func (store *Store) SetHead(ctx context.Context, newTs *block.TipSet) error {
	log.Infof("SetHead %s", newTs.String())

	// Add logging to debug sporadic test failure.
	if !newTs.Defined() {
		log.Errorf("publishing empty tipset")
		log.Error(debug.Stack())
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

	//todo wrap by go function
	Reverse(added)
	Reverse(dropped)

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
			case n := <-store.reorgNotifeeCh:
				notifees = append(notifees, n)

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
	store.reorgNotifeeCh <- f
}

// ReadOnlyStateStore provides a read-only IPLD store for access to chain state.
func (store *Store) ReadOnlyStateStore() util.ReadOnlyIpldStore {
	return util.ReadOnlyIpldStore{IpldStore: store.stateAndBlockSource.cborStore}
}

// writeHead writes the given cid set as head to disk.
func (store *Store) writeHead(ctx context.Context, cids block.TipSetKey) error {
	log.Debugf("WriteHead %s", cids.String())
	buf := new(bytes.Buffer)
	err := cids.MarshalCBOR(buf)
	if err != nil {
		return err
	}

	return store.ds.Put(HeadKey, buf.Bytes())
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

	metadata := TsState{
		StateRoot: tsm.TipSetStateRoot,
		Reciepts:  tsm.TipSetReceipts,
	}
	buf := new(bytes.Buffer)
	err := metadata.MarshalCBOR(buf)
	if err != nil {
		return err
	}
	// datastore keeps key:stateRoot (k,v) pairs.
	h, err := tsm.TipSet.Height()
	if err != nil {
		return err
	}
	key := datastore.NewKey(makeKey(tsm.TipSet.String(), h))
	return store.ds.Put(key, buf.Bytes())
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
func (store *Store) GetHead() *block.TipSet {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.head.Defined() {
		return block.UndefTipSet
	}

	return store.head
}

// GenesisCid returns the genesis cid of the chain tracked by the default store.
func (store *Store) GenesisCid() cid.Cid {
	return store.genesis
}

func (store *Store) GenesisRootCid() cid.Cid {
	genesis, _ := store.stateAndBlockSource.GetBlock(context.TODO(), store.GenesisCid())
	return genesis.ParentStateRoot
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

	log.Info("import height: ", root.EnsureHeight(), " root: ", root.At(0).ParentStateRoot, " parents: ", root.At(0).Parents)
	parentTipset, err := store.GetTipSet(parent)
	if err != nil {
		return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
	}
	err = store.PutTipSetMetadata(context.Background(), &TipSetMetadata{
		TipSetStateRoot: root.At(0).ParentStateRoot,
		TipSet:          parentTipset,
		TipSetReceipts:  root.At(0).ParentMessageReceipts,
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
			TipSetStateRoot: curTipset.At(0).ParentStateRoot,
			TipSet:          curParentTipset,
			TipSetReceipts:  curTipset.At(0).ParentMessageReceipts,
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
	log.Infof("WriteCheckPoint %v", cids)
	buf := new(bytes.Buffer)
	err := cids.MarshalCBOR(buf)
	if err != nil {
		return err
	}
	return store.ds.Put(CheckPoint, buf.Bytes())
}

func (store *Store) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st state.Tree) (CirculatingSupply, error) {
	return store.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, height, st)
}

func (store *Store) StateCirculatingSupply(ctx context.Context, tsk block.TipSetKey) (abi.TokenAmount, error) {
	ts, err := store.GetTipSet(tsk)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	root, err := store.GetTipSetStateRoot(ts)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	sTree, err := state.LoadState(ctx, store.stateAndBlockSource.cborStore, root)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	return store.getCirculatingSupply(ctx, ts.EnsureHeight(), sTree)
}

func (store *Store) getCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st state.Tree) (abi.TokenAmount, error) {
	adtStore := adt.WrapStore(ctx, store.stateAndBlockSource.cborStore)
	circ := big.Zero()
	unCirc := big.Zero()
	err := st.ForEach(func(a address.Address, actor *types.Actor) error {
		switch {
		case actor.Balance.IsZero():
			// Do nothing for zero-balance actors
			break
		case a == _init.Address ||
			a == reward.Address ||
			a == verifreg.Address ||
			// The power actor itself should never receive funds
			a == power.Address ||
			a == builtin.SystemActorAddr ||
			a == builtin.CronActorAddr ||
			a == builtin.BurntFundsActorAddr ||
			a == builtin.SaftAddress ||
			a == builtin.ReserveAddress:

			unCirc = big.Add(unCirc, actor.Balance)

		case a == market.Address:
			mst, err := market.Load(adtStore, actor)
			if err != nil {
				return err
			}

			lb, err := mst.TotalLocked()
			if err != nil {
				return err
			}

			circ = big.Add(circ, big.Sub(actor.Balance, lb))
			unCirc = big.Add(unCirc, lb)

		case builtin.IsAccountActor(actor.Code) || builtin.IsPaymentChannelActor(actor.Code):
			circ = big.Add(circ, actor.Balance)

		case builtin.IsStorageMinerActor(actor.Code):
			mst, err := miner.Load(adtStore, actor)
			if err != nil {
				return err
			}

			ab, err := mst.AvailableBalance(actor.Balance)

			if err == nil {
				circ = big.Add(circ, ab)
				unCirc = big.Add(unCirc, big.Sub(actor.Balance, ab))
			} else {
				// Assume any error is because the miner state is "broken" (lower actor balance than locked funds)
				// In this case, the actor's entire balance is considered "uncirculating"
				unCirc = big.Add(unCirc, actor.Balance)
			}

		case builtin.IsMultisigActor(actor.Code):
			mst, err := multisig.Load(adtStore, actor)
			if err != nil {
				return err
			}

			lb, err := mst.LockedBalance(height)
			if err != nil {
				return err
			}

			ab := big.Sub(actor.Balance, lb)
			circ = big.Add(circ, big.Max(ab, big.Zero()))
			unCirc = big.Add(unCirc, big.Min(actor.Balance, lb))
		default:
			return xerrors.Errorf("unexpected actor: %s", a)
		}

		return nil
	})

	if err != nil {
		return abi.TokenAmount{}, err
	}

	total := big.Add(circ, unCirc)
	if !total.Equals(crypto.TotalFilecoinInt) {
		return abi.TokenAmount{}, xerrors.Errorf("total filecoin didn't add to expected amount: %s != %s", total, crypto.TotalFilecoinInt)
	}

	return circ, nil
}

// GetCheckPoint get the check point from store or disk.
func (store *Store) GetCheckPoint() block.TipSetKey {
	return store.checkPoint
}

// Stop stops all activities and cleans up.
func (store *Store) Stop() {
	store.headEvents.Shutdown()
}

func (store *Store) ReorgOps(a, b *block.TipSet) ([]*block.TipSet, []*block.TipSet, error) {
	return ReorgOps(store.GetTipSet, a, b)
}

func ReorgOps(lts func(block.TipSetKey) (*block.TipSet, error), a, b *block.TipSet) ([]*block.TipSet, []*block.TipSet, error) {
	left := a
	right := b

	var leftChain, rightChain []*block.TipSet
	for !left.Equals(right) {
		lh, _ := left.Height()
		rh, _ := right.Height()
		if lh > rh {
			leftChain = append(leftChain, left)
			lKey, _ := left.Parents()
			par, err := lts(lKey)
			if err != nil {
				return nil, nil, err
			}

			left = par
		} else {
			rightChain = append(rightChain, right)
			rKey, _ := right.Parents()
			par, err := lts(rKey)
			if err != nil {
				log.Infof("failed to fetch right.Parents: %s", err)
				return nil, nil, err
			}

			right = par
		}
	}

	return leftChain, rightChain, nil

}

func (store *Store) PutMessage(m storable) (cid.Cid, error) {
	return PutMessage(store.bsstore, m)
}

func (store *Store) PutTipset(ctx context.Context, ts *block.TipSet) error {
	_, err := store.stateAndBlockSource.cborStore.Put(ctx, ts)

	return err
}

func (cs *Store) Blockstore() blockstore.Blockstore { // nolint
	return cs.bsstore
}
