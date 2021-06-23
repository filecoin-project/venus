package chain

import (
	"bytes"
	"context"
	"io"
	"os"
	"runtime/debug"
	"sync"

	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/state"

	"github.com/cskr/pubsub"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	blockadt "github.com/filecoin-project/specs-actors/actors/util/adt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/config"
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
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util"
)

// HeadChangeTopic is the topic used to publish new heads.
const (
	HeadChangeTopic = "headchange"
	HCRevert        = "revert"
	HCApply         = "apply"
	HCCurrent       = "current"
)

// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
var ErrNoMethod = errors.New("no method")

// ErrNoActorImpl is returned by Get when the actor implementation doesn't exist, eg
// the actor address is an empty actor, an address that has received a transfer of FIL
// but hasn't yet been upgraded to an account actor. (The actor implementation might
// also genuinely be missing, which is not expected.)
var ErrNoActorImpl = errors.New("no actor implementation")

// GenesisKey is the key at which the genesis Cid is written in the datastore.
var GenesisKey = datastore.NewKey("/consensus/genesisCid")

var log = logging.Logger("chain.store")

// HeadKey is the key at which the head tipset cid's are written in the datastore.
var HeadKey = datastore.NewKey("/chain/heaviestTipSet")

var ErrNotifeeDone = errors.New("notifee is done and should be removed")

type loadTipSetFunc func(types.TipSetKey) (*types.TipSet, error)

// ReorgNotifee represents a callback that gets called upon reorgs.
type ReorgNotifee func(rev, app []*types.TipSet) error

type reorg struct {
	old []*types.TipSet
	new []*types.TipSet
}

type HeadChange struct {
	Type string
	Val  *types.TipSet
}

// CheckPoint is the key which the check-point written in the datastore.
var CheckPoint = datastore.NewKey("/chain/checkPoint")

//TSState export this func is just for gen cbor tool to work
type TSState struct {
	StateRoot cid.Cid
	Receipts  cid.Cid
}

func ActorStore(ctx context.Context, bs blockstore.Blockstore) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(bs))
}

// Store is a generic implementation of the Store interface.
// It works(tm) for now.
type Store struct {
	// ipldSource is a wrapper around ipld storage.  It is used
	// for reading filecoin block and state objects kept by the node.
	stateAndBlockSource cbor.IpldStore

	bsstore blockstore.Blockstore

	// ds is the datastore for the chain's private metadata which consists
	// of the tipset key to state root cid mapping, and the heaviest tipset
	// key.
	ds repo.Datastore

	// genesis is the CID of the genesis block.
	genesis cid.Cid
	// head is the tipset at the head of the best known chain.
	head *types.TipSet

	checkPoint types.TipSetKey
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
	forkConfig *config.ForkUpgradeConfig,
	genesisCid cid.Cid,
) *Store {
	tsCache, _ := lru.NewARC(10000)
	store := &Store{
		stateAndBlockSource: cst,
		ds:                  ds,
		bsstore:             bsstore,
		headEvents:          pubsub.New(64),

		checkPoint:     types.UndefTipSet.Key(),
		genesis:        genesisCid,
		reorgNotifeeCh: make(chan ReorgNotifee),
		tsCache:        tsCache,
	}
	//todo cycle reference , may think a better idea
	store.tipIndex = NewTipStateCache(store)
	store.chainIndex = NewChainIndex(store.GetTipSet)
	store.circulatingSupplyCalculator = NewCirculatingSupplyCalculator(bsstore, store, forkConfig)

	val, err := store.ds.Get(CheckPoint)
	if err != nil {
		store.checkPoint = types.NewTipSetKey(genesisCid)
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

	headTSKey, err := store.loadHead()
	if err != nil {
		return err
	}

	headTS, err := LoadTipSetBlocks(ctx, store, headTSKey)
	if err != nil {
		return errors.Wrap(err, "error loading head tipset")
	}

	latestHeight := headTS.At(0).Height
	loopBack := latestHeight - policy.ChainFinality
	log.Infof("start loading chain at tipset: %s, height: %d", headTSKey.String(), latestHeight)

	// Provide tipsets directly from the block store, not from the tipset index which is
	// being rebuilt by this traversal.
	tipsetProvider := TipSetProviderFromBlocks(ctx, store)
	for iterator := IterAncestors(ctx, tipsetProvider, headTS); !iterator.Complete(); err = iterator.Next() {
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

		if ts.Height() <= loopBack {
			break
		}
	}

	log.Infof("finished loading %d tipsets from %s", latestHeight, headTS.String())
	// Set actual head.
	return store.SetHead(ctx, headTS)
}

// loadHead loads the latest known head from disk.
func (store *Store) loadHead() (types.TipSetKey, error) {
	var emptyCidSet types.TipSetKey
	tskBytes, err := store.ds.Get(HeadKey)
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to read HeadKey")
	}

	var tsk types.TipSetKey
	err = tsk.UnmarshalCBOR(bytes.NewReader(tskBytes))
	if err != nil {
		return emptyCidSet, errors.Wrap(err, "failed to cast headCids")
	}

	return tsk, nil
}

//LoadTipsetMetadata load tipset status (state root and reciepts)
func (store *Store) LoadTipsetMetadata(ts *types.TipSet) (*TipSetMetadata, error) {
	h := ts.Height()
	key := datastore.NewKey(makeKey(ts.String(), h))
	tsStateBytes, err := store.ds.Get(key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read tipset key %s", ts.String())
	}

	var metadata TSState
	err = metadata.UnmarshalCBOR(bytes.NewReader(tsStateBytes))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode tip set metadata %s", ts.String())
	}
	return &TipSetMetadata{
		TipSet:          ts,
		TipSetStateRoot: metadata.StateRoot,
		TipSetReceipts:  metadata.Receipts,
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

// Ls returns an iterator over tipsets from head to genesis.
func (store *Store) Ls(ctx context.Context, fromTS *types.TipSet, count int) ([]*types.TipSet, error) {
	tipsets := []*types.TipSet{fromTS}
	fromKey := fromTS.Parents()
	for i := 0; i < count-1; i++ {
		ts, err := store.GetTipSet(fromKey)
		if err != nil {
			return nil, err
		}
		tipsets = append(tipsets, ts)
		fromKey = ts.Parents()
	}
	types.ReverseTipSet(tipsets)
	return tipsets, nil
}

// GetBlock returns the block identified by `cid`.
func (store *Store) GetBlock(ctx context.Context, blockID cid.Cid) (*types.BlockHeader, error) {
	var block types.BlockHeader
	err := store.stateAndBlockSource.Get(ctx, blockID, &block)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %s", blockID.String())
	}
	return &block, nil
}

// GetBlock returns the block identified by `cid`.
func (store *Store) PutObject(ctx context.Context, obj interface{}) (cid.Cid, error) {
	return store.stateAndBlockSource.Put(ctx, obj)
}

// GetTipSet returns the tipset identified by `key`.
func (store *Store) GetTipSet(key types.TipSetKey) (*types.TipSet, error) {
	if key.IsEmpty() {
		return store.GetHead(), nil
	}
	var blks []*types.BlockHeader
	val, has := store.tsCache.Get(key)
	if has {
		return val.(*types.TipSet), nil
	}

	for _, id := range key.Cids() {
		blk, err := store.GetBlock(context.TODO(), id)

		if err != nil {
			return nil, err
		}
		blks = append(blks, blk)
	}
	ts, err := types.NewTipSet(blks...)
	if err != nil {
		return nil, err
	}
	store.tsCache.Add(key, ts)
	return ts, nil
}

// GetTipSetByHeight looks back for a tipset at the specified epoch.
// If there are no blocks at the specified epoch, a tipset at an earlier epoch
// will be returned.
func (store *Store) GetTipSetByHeight(ctx context.Context, ts *types.TipSet, h abi.ChainEpoch, prev bool) (*types.TipSet, error) {
	if ts == nil {
		ts = store.head
	}

	if h > ts.Height() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if h == ts.Height() {
		return ts, nil
	}

	lbts, err := store.chainIndex.GetTipSetByHeight(ctx, ts, h)
	if err != nil {
		return nil, err
	}

	if lbts.Height() < h {
		log.Warnf("chain index returned the wrong tipset at height %d, using slow retrieval", h)
		lbts, err = store.chainIndex.GetTipsetByHeightWithoutCache(ts, h)
		if err != nil {
			return nil, err
		}
	}

	if lbts.Height() == h || !prev {
		return lbts, nil
	}

	return store.GetTipSet(lbts.Parents())
}

// GetTipSetState returns the aggregate state of the tipset identified by `key`.
func (store *Store) GetTipSetState(ctx context.Context, ts *types.TipSet) (tree.Tree, error) {
	if ts == nil {
		ts = store.head
	}
	stateCid, err := store.tipIndex.GetTipSetStateRoot(ts)
	if err != nil {
		return nil, err
	}
	return tree.LoadState(ctx, store.stateAndBlockSource, stateCid)
}

// GetGenesisBlock returns the genesis block held by the chain store.
func (store *Store) GetGenesisBlock(ctx context.Context) (*types.BlockHeader, error) {
	return store.GetBlock(ctx, store.GenesisCid())
}

// GetTipSetStateRoot returns the aggregate state root CID of the tipset identified by `key`.
func (store *Store) GetTipSetStateRoot(key *types.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetStateRoot(key)
}

// GetTipSetReceiptsRoot returns the root CID of the message receipts for the tipset identified by `key`.
func (store *Store) GetTipSetReceiptsRoot(key *types.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetReceiptsRoot(key)
}

// HasTipSetAndState returns true iff the default store's tipindex is indexing
// the tipset identified by `key`.
func (store *Store) HasTipSetAndState(ctx context.Context, ts *types.TipSet) bool {
	return store.tipIndex.Has(ts)
}

//GetLatestBeaconEntry get latest beacon from the height. there're no beacon values in the block, try to
//get beacon in the parents tipset. the max find depth is 20.
func (store *Store) GetLatestBeaconEntry(ts *types.TipSet) (*types.BeaconEntry, error) {
	cur := ts
	for i := 0; i < 20; i++ {
		cbe := cur.At(0).BeaconEntries
		if len(cbe) > 0 {
			return cbe[len(cbe)-1], nil
		}

		if cur.Height() == 0 {
			return nil, xerrors.Errorf("made it back to genesis block without finding beacon entry")
		}

		next, err := store.GetTipSet(cur.Parents())
		if err != nil {
			return nil, xerrors.Errorf("failed to load parents when searching back for latest beacon entry: %w", err)
		}
		cur = next
	}

	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return &types.BeaconEntry{
			Data: []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
		}, nil
	}

	return nil, xerrors.Errorf("found NO beacon entries in the 20 blocks prior to given tipset")
}

// nolint
func (store *Store) walkBack(from *types.TipSet, to abi.ChainEpoch) (*types.TipSet, error) {
	if to > from.Height() {
		return nil, xerrors.Errorf("looking for tipset with height greater than start point")
	}

	if to == from.Height() {
		return from, nil
	}

	ts := from

	for {
		pts, err := store.GetTipSet(ts.Parents())
		if err != nil {
			return nil, err
		}

		if to > pts.Height() {
			// in case pts is lower than the epoch we're looking for (null blocks)
			// return a tipset above that height
			return ts, nil
		}
		if to == pts.Height() {
			return pts, nil
		}

		ts = pts
	}
}

// GetSiblingState returns the the tipsets and states tracked by
// the default store's tipIndex that have parents identified by `parentKey`.
func (store *Store) GetSiblingState(ts *types.TipSet) ([]*TipSetMetadata, error) {
	return store.tipIndex.GetSiblingState(ts)
}

// HasSiblingState returns true if the default store's tipindex
// contains any tipset identified by `parentKey`.
func (store *Store) HasSiblingState(ts *types.TipSet) bool {
	return store.tipIndex.HasSiblingState(ts)
}

// SetHead sets the passed in tipset as the new head of this chain.
func (store *Store) SetHead(ctx context.Context, newTS *types.TipSet) error {
	log.Infof("SetHead %s %d", newTS.String(), newTS.Height())
	// Add logging to debug sporadic test failure.
	if !newTS.Defined() {
		log.Errorf("publishing empty tipset")
		log.Error(debug.Stack())
		return nil
	}

	//reorg tipset
	dropped, added, update, err := func() ([]*types.TipSet, []*types.TipSet, bool, error) {
		var dropped []*types.TipSet
		var added []*types.TipSet
		var err error
		store.mu.Lock()
		defer store.mu.Unlock()

		if store.head != nil {
			if store.head.Equals(newTS) {
				return nil, nil, false, nil
			}
			//reorg
			oldHead := store.head
			dropped, added, err = CollectTipsToCommonAncestor(ctx, store, oldHead, newTS)
			if err != nil {
				return nil, nil, false, err
			}
		} else {
			added = []*types.TipSet{newTS}
		}

		// Ensure consistency by storing this new head on disk.
		if errInner := store.writeHead(ctx, newTS.Key()); errInner != nil {
			return nil, nil, false, errors.Wrap(errInner, "failed to write new Head to datastore")
		}
		store.head = newTS
		return dropped, added, true, nil
	}()

	if err != nil {
		return err
	}

	if !update {
		return nil
	}

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
	headChangeNotifee := func(rev, app []*types.TipSet) error {
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

// SubHeadChanges returns channel with chain head updates.
// First message is guaranteed to be of len == 1, and type == 'current'.
// Then event in the message may be HCApply and HCRevert.
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

//SubscribeHeadChanges subscribe head change event
func (store *Store) SubscribeHeadChanges(f ReorgNotifee) {
	store.reorgNotifeeCh <- f
}

// ReadOnlyStateStore provides a read-only IPLD store for access to chain state.
func (store *Store) ReadOnlyStateStore() util.ReadOnlyIpldStore {
	return util.ReadOnlyIpldStore{IpldStore: store.stateAndBlockSource}
}

// writeHead writes the given cid set as head to disk.
func (store *Store) writeHead(ctx context.Context, cids types.TipSetKey) error {
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

	metadata := TSState{
		StateRoot: tsm.TipSetStateRoot,
		Receipts:  tsm.TipSetReceipts,
	}
	buf := new(bytes.Buffer)
	err := metadata.MarshalCBOR(buf)
	if err != nil {
		return err
	}
	// datastore keeps key:stateRoot (k,v) pairs.
	h := tsm.TipSet.Height()
	key := datastore.NewKey(makeKey(tsm.TipSet.String(), h))
	return store.ds.Put(key, buf.Bytes())
}

// deleteTipSetMetadata delete the state root id from the datastore for the tipset key.
func (store *Store) deleteTipSetMetadata(ts *types.TipSet) error { // nolint
	h := ts.Height()

	key := datastore.NewKey(makeKey(ts.String(), h))
	return store.ds.Delete(key)
}

// GetHead returns the current head tipset cids.
func (store *Store) GetHead() *types.TipSet {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.head.Defined() {
		return types.UndefTipSet
	}

	return store.head
}

// GenesisCid returns the genesis cid of the chain tracked by the default store.
func (store *Store) GenesisCid() cid.Cid {
	return store.genesis
}

// GenesisRootCid returns the genesis root cid of the chain tracked by the default store.
func (store *Store) GenesisRootCid() cid.Cid {
	genesis, _ := store.GetBlock(context.TODO(), store.GenesisCid())
	return genesis.ParentStateRoot
}

//Import import a car file into local db
func (store *Store) Import(r io.Reader) (*types.TipSet, error) {
	header, err := car.LoadCar(store.bsstore, r)
	if err != nil {
		return nil, xerrors.Errorf("loadcar failed: %w", err)
	}

	root, err := store.GetTipSet(types.NewTipSetKey(header.Roots...))
	if err != nil {
		return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
	}

	parent := root.Parents()

	//Notice here is different with lotus, because the head tipset in lotus is not computed,
	//but in venus the head tipset is computed, so here we will fallback a pre tipset
	//and the chain store must has a metadata for each tipset, below code is to build the tipset metadata
	log.Info("import height: ", root.Height(), " root: ", root.At(0).ParentStateRoot, " parents: ", root.At(0).Parents)
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
		curTipsetKey := curTipset.Parents()
		curParentTipset, err := store.GetTipSet(curTipsetKey)
		if err != nil {
			return nil, xerrors.Errorf("failed to load root tipset from chainfile: %w", err)
		}

		if curParentTipset.Height() == 0 {
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

// SetCheckPoint set current checkpoint
func (store *Store) SetCheckPoint(checkPoint types.TipSetKey) {
	store.checkPoint = checkPoint
}

// WriteCheckPoint writes the given cids to disk.
func (store *Store) WriteCheckPoint(ctx context.Context, cids types.TipSetKey) error {
	log.Infof("WriteCheckPoint %v", cids)
	buf := new(bytes.Buffer)
	err := cids.MarshalCBOR(buf)
	if err != nil {
		return err
	}
	return store.ds.Put(CheckPoint, buf.Bytes())
}

func (store *Store) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (CirculatingSupply, error) {
	return store.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, height, st)
}

//StateCirculatingSupply get circulate supply at specify epoch
func (store *Store) StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error) {
	ts, err := store.GetTipSet(tsk)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	root, err := store.GetTipSetStateRoot(ts)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	sTree, err := tree.LoadState(ctx, store.stateAndBlockSource, root)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	return store.getCirculatingSupply(ctx, ts.Height(), sTree)
}

func (store *Store) getCirculatingSupply(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (abi.TokenAmount, error) {
	adtStore := adt.WrapStore(ctx, store.stateAndBlockSource)
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
	if !total.Equals(types.TotalFilecoinInt) {
		return abi.TokenAmount{}, xerrors.Errorf("total filecoin didn't add to expected amount: %s != %s", total, types.TotalFilecoinInt)
	}

	return circ, nil
}

// GetCheckPoint get the check point from store or disk.
func (store *Store) GetCheckPoint() types.TipSetKey {
	return store.checkPoint
}

// Stop stops all activities and cleans up.
func (store *Store) Stop() {
	store.headEvents.Shutdown()
}

// ReorgOps used to reorganize the blockchain. Whenever a new tipset is approved,
// the new tipset compared with the local tipset to obtain which tipset need to be revert and which tipsets are applied
func (store *Store) ReorgOps(a, b *types.TipSet) ([]*types.TipSet, []*types.TipSet, error) {
	return ReorgOps(store.GetTipSet, a, b)
}

// todo should move this code into store.ReorgOps. anywhere use this function should invoke store.ReorgOps
func ReorgOps(lts func(types.TipSetKey) (*types.TipSet, error), a, b *types.TipSet) ([]*types.TipSet, []*types.TipSet, error) {
	left := a
	right := b

	var leftChain, rightChain []*types.TipSet
	for !left.Equals(right) {
		lh := left.Height()
		rh := right.Height()
		if lh > rh {
			leftChain = append(leftChain, left)
			lKey := left.Parents()
			par, err := lts(lKey)
			if err != nil {
				return nil, nil, err
			}

			left = par
		} else {
			rightChain = append(rightChain, right)
			rKey := right.Parents()
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

// PutMessage put message in local db
func (store *Store) PutMessage(m storable) (cid.Cid, error) {
	return PutMessage(store.bsstore, m)
}

// Blockstore return local blockstore
// todo remove this method, and code that need blockstore should get from blockstore submodule
func (store *Store) Blockstore() blockstore.Blockstore { // nolint
	return store.bsstore
}

// GetParentReceipt get the receipt of parent tipset at specify message slot
func (store *Store) GetParentReceipt(b *types.BlockHeader, i int) (*types.MessageReceipt, error) {
	ctx := context.TODO()
	// block headers use adt0, for now.
	a, err := blockadt.AsArray(adt.WrapStore(ctx, store.stateAndBlockSource), b.ParentMessageReceipts)
	if err != nil {
		return nil, xerrors.Errorf("amt load: %w", err)
	}

	var r types.MessageReceipt
	if found, err := a.Get(uint64(i), &r); err != nil {
		return nil, err
	} else if !found {
		return nil, xerrors.Errorf("failed to find receipt %d", i)
	}

	return &r, nil
}

// GetLookbackTipSetForRound get loop back tipset and state root
func (store *Store) GetLookbackTipSetForRound(ctx context.Context, ts *types.TipSet, round abi.ChainEpoch, version network.Version) (*types.TipSet, cid.Cid, error) {
	var lbr abi.ChainEpoch

	lb := policy.GetWinningPoStSectorSetLookback(version)
	if round > lb {
		lbr = round - lb
	}

	// more null blocks than our lookback
	h := ts.Height()
	if lbr >= h {
		// This should never happen at this point, but may happen before
		// network version 3 (where the lookback was only 10 blocks).
		st, err := store.GetTipSetStateRoot(ts)
		if err != nil {
			return nil, cid.Undef, err
		}
		return ts, st, nil
	}

	// Get the tipset after the lookback tipset, or the next non-null one.
	nextTS, err := store.GetTipSetByHeight(ctx, ts, lbr+1, false)
	if err != nil {
		return nil, cid.Undef, xerrors.Errorf("failed to get lookback tipset+1: %v", err)
	}

	nextTh := nextTS.Height()
	if lbr > nextTh {
		return nil, cid.Undef, xerrors.Errorf("failed to find non-null tipset %s (%d) which is known to exist, found %s (%d)", ts.Key(), h, nextTS.Key(), nextTh)
	}

	pKey := nextTS.Parents()
	lbts, err := store.GetTipSet(pKey)
	if err != nil {
		return nil, cid.Undef, xerrors.Errorf("failed to resolve lookback tipset: %v", err)
	}

	return lbts, nextTS.Blocks()[0].ParentStateRoot, nil
}

// Randomness

// SampleChainRandomness computes randomness seeded by a ticket from the chain `head` at `sampleHeight`.
func (store *Store) SampleChainRandomness(ctx context.Context, tsk types.TipSetKey, tag acrypto.DomainSeparationTag,
	epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	genBlk, err := store.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}

	rnd := ChainRandomnessSource{Sampler: NewRandomnessSamplerAtTipSet(store, genBlk.Ticket, tsk)}
	return rnd.GetRandomnessFromTickets(ctx, tag, epoch, entropy)
}

func (store *Store) ChainGetRandomnessFromBeacon(ctx context.Context, tsk types.TipSetKey, personalization acrypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	genBlk, err := store.GetGenesisBlock(ctx)
	if err != nil {
		return nil, err
	}
	rnd := ChainRandomnessSource{Sampler: NewRandomnessSamplerAtTipSet(store, genBlk.Ticket, tsk)}
	return rnd.GetRandomnessFromBeacon(ctx, personalization, randEpoch, entropy)
}

//Actor

// LsActors returns a channel with actors from the latest state on the chain
func (store *Store) LsActors(ctx context.Context) (map[address.Address]*types.Actor, error) {
	st, err := store.GetTipSetState(ctx, store.head)
	if err != nil {
		return nil, err
	}

	result := make(map[address.Address]*types.Actor)
	err = st.ForEach(func(key address.Address, a *types.Actor) error {
		result[key] = a
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetActorAt returns an actor at a specified tipset key.
func (store *Store) GetActorAt(ctx context.Context, ts *types.TipSet, addr address.Address) (*types.Actor, error) {
	st, err := store.GetTipSetState(ctx, ts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load latest state")
	}

	idAddr, err := store.LookupID(ctx, ts, addr)
	if err != nil {
		return nil, err
	}

	actr, found, err := st.GetActor(ctx, idAddr)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, types.ErrActorNotFound
	}
	return actr, nil
}

// LookupID resolves ID address for actor
func (store *Store) LookupID(ctx context.Context, ts *types.TipSet, addr address.Address) (address.Address, error) {
	st, err := store.GetTipSetState(ctx, ts)
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to load latest state")
	}

	return st.LookupID(addr)
}

// ResolveToKeyAddr get key address of specify address.
//if ths addr is bls/secpk address, return directly, other get the pubkey and generate address
func (store *Store) ResolveToKeyAddr(ctx context.Context, ts *types.TipSet, addr address.Address) (address.Address, error) {
	st, err := store.StateView(ts)
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to load latest state")
	}

	return st.ResolveToKeyAddr(ctx, addr)
}

// StateView return state view at ts epoch
func (store *Store) StateView(ts *types.TipSet) (*state.View, error) {
	if ts == nil {
		ts = store.head
	}
	root, err := store.GetTipSetStateRoot(ts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", ts.Key().String())
	}

	return state.NewView(store.stateAndBlockSource, root), nil
}

// AccountView return account view at ts state
func (store *Store) AccountView(ts *types.TipSet) (state.AccountView, error) {
	if ts == nil {
		ts = store.head
	}
	root, err := store.GetTipSetStateRoot(ts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", ts.Key().String())
	}

	return state.NewView(store.stateAndBlockSource, root), nil
}

// ParentStateView get parent state view of ts
func (store *Store) ParentStateView(ts *types.TipSet) (*state.View, error) {
	return state.NewView(store.stateAndBlockSource, ts.At(0).ParentStateRoot), nil
}

// Store wrap adt store
func (store *Store) Store(ctx context.Context) adt.Store {
	return adt.WrapStore(ctx, cbor.NewCborStore(store.bsstore))
}
