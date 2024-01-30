package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/arc/v2"
	"github.com/ipld/go-car"
	carutil "github.com/ipld/go-car/util"
	carv2 "github.com/ipld/go-car/v2"
	"github.com/multiformats/go-multicodec"
	"github.com/pkg/errors"
	cbg "github.com/whyrusleeping/cbor-gen"
	"go.opencensus.io/trace"

	blockstore "github.com/ipfs/boxo/blockstore"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/pubsub"
	blockadt "github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/metrics/tracing"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util"

	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	_init "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/multisig"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// GenesisKey is the key at which the genesis Cid is written in the datastore.
var GenesisKey = datastore.NewKey("/consensus/genesisCid")

var log = logging.Logger("chain.store")

// HeadKey is the key at which the head tipset cid's are written in the datastore.
var HeadKey = datastore.NewKey("/chain/heaviestTipSet")

var ErrNotifeeDone = errors.New("notifee is done and should be removed")

type loadTipSetFunc func(context.Context, types.TipSetKey) (*types.TipSet, error)

// ReorgNotifee represents a callback that gets called upon reorgs.
type ReorgNotifee func(rev, app []*types.TipSet) error

var DefaultTipsetLruCacheSize = 10000

type reorg struct {
	old []*types.TipSet
	new []*types.TipSet
}

// CheckPoint is the key which the check-point written in the datastore.
var CheckPoint = datastore.NewKey("/chain/checkPoint")

// TSState export this func is just for gen cbor tool to work
type TSState struct {
	StateRoot cid.Cid
	Receipts  cid.Cid
}

type WeightFunc func(context.Context, cbor.IpldStore, *types.TipSet) (big.Int, error)

// Store is a generic implementation of the Store interface.
// It works(tm) for now.
type Store struct {
	// ipldSource is a wrapper around ipld storage.  It is used
	// for reading filecoin block and state objects kept by the node.
	stateAndBlockSource cbor.IpldStore

	bsstore blockstoreutil.Blockstore

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

	circulatingSupplyCalculator ICirculatingSupplyCalcualtor

	chainIndex *ChainIndex

	reorgCh        chan reorg
	reorgNotifeeCh chan ReorgNotifee

	tsCache *arc.ARCCache[types.TipSetKey, *types.TipSet]

	tstLk   sync.Mutex
	tipsets map[abi.ChainEpoch][]cid.Cid

	weight WeightFunc
}

// NewStore constructs a new default store.
func NewStore(chainDs repo.Datastore,
	bsstore blockstoreutil.Blockstore,
	genesisCid cid.Cid,
	circulatiingSupplyCalculator ICirculatingSupplyCalcualtor,
	weight WeightFunc,
) *Store {
	tsCache, _ := arc.NewARC[types.TipSetKey, *types.TipSet](DefaultTipsetLruCacheSize)
	store := &Store{
		stateAndBlockSource: cbor.NewCborStore(bsstore),
		ds:                  chainDs,
		bsstore:             bsstore,
		headEvents:          pubsub.New(64),

		checkPoint:     types.EmptyTSK,
		genesis:        genesisCid,
		reorgNotifeeCh: make(chan ReorgNotifee),
		tsCache:        tsCache,
		tipsets:        make(map[abi.ChainEpoch][]cid.Cid, constants.Finality),
		weight:         weight,
	}
	// todo cycle reference , may think a better idea
	store.tipIndex = NewTipStateCache(store)
	store.chainIndex = NewChainIndex(store.GetTipSet)
	store.circulatingSupplyCalculator = circulatiingSupplyCalculator

	val, err := store.ds.Get(context.TODO(), CheckPoint)
	if err != nil {
		store.checkPoint = types.NewTipSetKey(genesisCid)
	} else {
		_ = store.checkPoint.UnmarshalCBOR(bytes.NewReader(val)) //nolint:staticcheck
	}
	log.Infof("check point value: %v", store.checkPoint)

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

	var headTS *types.TipSet

	if headTS, err = store.loadHead(ctx); err != nil {
		return err
	}

	if headTS.Height() == 0 {
		return store.SetHead(ctx, headTS)
	}

	latestHeight := headTS.At(0).Height
	loopBack := latestHeight - policy.ChainFinality
	log.Infof("start loading chain at tipset: %s, height: %d", headTS.Key(), headTS.Height())

	// `Metadata` of head may not exist, this is okay, its parent's `Meta` is surely exists.
	headParent, err := store.GetTipSet(ctx, headTS.Parents())
	if err != nil {
		return err
	}

	// Provide tipsets directly from the block store, not from the tipset index which is
	// being rebuilt by this traversal.
	tipsetProvider := TipSetProviderFromBlocks(ctx, store)
	for iterator := IterAncestors(ctx, tipsetProvider, headParent); !iterator.Complete(); err = iterator.Next(ctx) {
		if err != nil {
			return err
		}
		ts := iterator.Value()
		if ts.Height() == 0 {
			break
		}
		tipSetMetadata, err := store.LoadTipsetMetadata(ctx, ts)
		if err != nil {
			return err
		}

		store.tipIndex.Put(tipSetMetadata)

		if ts.Height() <= loopBack {
			break
		}
	}
	log.Infof("finished loading %d tipsets from %s", latestHeight, headTS.String())

	// Set actual head.
	return store.SetHead(ctx, headTS)
}

// loadHead loads the latest known head from disk.
func (store *Store) loadHead(ctx context.Context) (*types.TipSet, error) {
	tskBytes, err := store.ds.Get(ctx, HeadKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read HeadKey")
	}

	var tsk types.TipSetKey
	err = tsk.UnmarshalCBOR(bytes.NewReader(tskBytes))
	if err != nil {
		// todo: remove after nv18
		err = tsk.V0UnmarshalCBOR(bytes.NewReader(tskBytes))
		if err != nil {
			return nil, errors.Wrap(err, "failed to cast headCids")
		}
	}

	return store.GetTipSet(ctx, tsk)
}

// LoadTipsetMetadata load tipset status (state root and reciepts)
func (store *Store) LoadTipsetMetadata(ctx context.Context, ts *types.TipSet) (*TipSetMetadata, error) {
	h := ts.Height()
	key := datastore.NewKey(makeKey(ts.String(), h))

	tsStateBytes, err := store.ds.Get(ctx, key)
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
	store.tipIndex.Put(tsm)

	// Persist the state mapping.
	return store.writeTipSetMetadata(ctx, tsm)
}

// Ls returns an iterator over tipsets from head to genesis.
func (store *Store) Ls(ctx context.Context, fromTS *types.TipSet, count int) ([]*types.TipSet, error) {
	tipsets := []*types.TipSet{fromTS}
	fromKey := fromTS.Parents()
	for i := 0; i < count-1; i++ {
		ts, err := store.GetTipSet(ctx, fromKey)
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
func (store *Store) GetTipSet(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
	if key.IsEmpty() {
		return store.GetHead(), nil
	}

	if val, has := store.tsCache.Get(key); has {
		return val, nil
	}

	cids := key.Cids()
	blks := make([]*types.BlockHeader, len(cids))
	for idx, c := range cids {
		blk, err := store.GetBlock(ctx, c)
		if err != nil {
			return nil, err
		}

		blks[idx] = blk
	}

	ts, err := types.NewTipSet(blks)
	if err != nil {
		return nil, err
	}
	store.tsCache.Add(key, ts)

	return ts, nil
}

// GetTipSetByHeight looks back for a tipset at the specified epoch.
// In the case that the given height is a null round, the 'prev' flag
// selects the tipset before the null round if true, and the tipset following
// the null round if false.
func (store *Store) GetTipSetByHeight(ctx context.Context, ts *types.TipSet, h abi.ChainEpoch, prev bool) (*types.TipSet, error) {
	if h < 0 {
		return nil, fmt.Errorf("height %d is negative", h)
	}

	if ts == nil {
		ts = store.head
	}

	if h > ts.Height() {
		return nil, fmt.Errorf("looking for tipset with height greater than start point")
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
		lbts, err = store.chainIndex.GetTipsetByHeightWithoutCache(ctx, ts, h)
		if err != nil {
			return nil, err
		}
	}

	if lbts.Height() == h || !prev {
		return lbts, nil
	}

	return store.GetTipSet(ctx, lbts.Parents())
}

func (store *Store) GetTipSetByCid(ctx context.Context, c cid.Cid) (*types.TipSet, error) {
	blk, err := store.bsstore.Get(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("cannot find tipset with cid %s: %w", c, err)
	}

	tsk := new(types.TipSetKey)
	if err := tsk.UnmarshalCBOR(bytes.NewReader(blk.RawData())); err != nil {
		return nil, fmt.Errorf("cannot unmarshal block into tipset key: %w", err)
	}

	ts, err := store.GetTipSet(ctx, *tsk)
	if err != nil {
		return nil, fmt.Errorf("cannot get tipset from key: %w", err)
	}
	return ts, nil
}

// GetTipSetState returns the aggregate state of the tipset identified by `key`.
func (store *Store) GetTipSetState(ctx context.Context, ts *types.TipSet) (tree.Tree, error) {
	if ts == nil {
		ts = store.head
	}
	stateCid, err := store.tipIndex.GetTipSetStateRoot(ctx, ts)
	if err != nil {
		return nil, err
	}
	return tree.LoadState(ctx, store.stateAndBlockSource, stateCid)
}

// GetGenesisBlock returns the genesis block held by the chain store.
func (store *Store) GetGenesisBlock(ctx context.Context) (*types.BlockHeader, error) {
	return store.GetBlock(ctx, store.GenesisCid())
}

func (store *Store) PersistGenesisCID(ctx context.Context, blk *types.BlockHeader) error {
	data, err := json.Marshal(blk.Cid())
	if err != nil {
		return fmt.Errorf("failed to marshal genesis cid: %v", err)
	}

	return store.ds.Put(ctx, GenesisKey, data)
}

func GenesisBlock(ctx context.Context, chainDs datastore.Datastore, bs blockstoreutil.Blockstore) (types.BlockHeader, error) {
	bb, err := chainDs.Get(ctx, GenesisKey)
	if err != nil {
		return types.BlockHeader{}, fmt.Errorf("failed to read genesisKey: %v", err)
	}

	var c cid.Cid
	err = json.Unmarshal(bb, &c)
	if err != nil {
		return types.BlockHeader{}, fmt.Errorf("failed to cast genesisCid: %v", err)
	}

	var blk types.BlockHeader
	err = cbor.NewCborStore(bs).Get(ctx, c, &blk)
	if err != nil {
		return types.BlockHeader{}, fmt.Errorf("failed to read genesis block: %v", err)
	}

	return blk, nil
}

// GetTipSetStateRoot returns the aggregate state root CID of the tipset identified by `key`.
func (store *Store) GetTipSetStateRoot(ctx context.Context, key *types.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetStateRoot(ctx, key)
}

// GetTipSetReceiptsRoot returns the root CID of the message receipts for the tipset identified by `key`.
func (store *Store) GetTipSetReceiptsRoot(ctx context.Context, key *types.TipSet) (cid.Cid, error) {
	return store.tipIndex.GetTipSetReceiptsRoot(ctx, key)
}

func (store *Store) GetTipsetMetadata(ctx context.Context, ts *types.TipSet) (*TipSetMetadata, error) {
	tsStat, err := store.tipIndex.Get(ctx, ts)
	if err != nil {
		return nil, err
	}
	return &TipSetMetadata{
		TipSetStateRoot: tsStat.StateRoot,
		TipSet:          ts,
		TipSetReceipts:  tsStat.Receipts,
	}, nil
}

// HasTipSetAndState returns true iff the default store's tipindex is indexing
// the tipset identified by `key`.
func (store *Store) HasTipSetAndState(ctx context.Context, ts *types.TipSet) bool {
	return store.tipIndex.Has(ctx, ts)
}

// GetLatestBeaconEntry get latest beacon from the height. there're no beacon values in the block, try to
// get beacon in the parents tipset. the max find depth is 20.
func (store *Store) GetLatestBeaconEntry(ctx context.Context, ts *types.TipSet) (*types.BeaconEntry, error) {
	cur := ts
	for i := 0; i < 20; i++ {
		cbe := cur.At(0).BeaconEntries
		if len(cbe) > 0 {
			return &cbe[len(cbe)-1], nil
		}

		if cur.Height() == 0 {
			return nil, fmt.Errorf("made it back to genesis block without finding beacon entry")
		}

		next, err := store.GetTipSet(ctx, cur.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to load parents when searching back for latest beacon entry: %w", err)
		}
		cur = next
	}

	if os.Getenv("VENUS_IGNORE_DRAND") == "_yes_" {
		return &types.BeaconEntry{
			Data: []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9},
		}, nil
	}

	return nil, fmt.Errorf("found NO beacon entries in the 20 blocks prior to given tipset")
}

// nolint
func (store *Store) walkBack(ctx context.Context, from *types.TipSet, to abi.ChainEpoch) (*types.TipSet, error) {
	if to > from.Height() {
		return nil, fmt.Errorf("looking for tipset with height greater than start point")
	}

	if to == from.Height() {
		return from, nil
	}

	ts := from

	for {
		pts, err := store.GetTipSet(ctx, ts.Parents())
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

// updateHead sets the passed in tipset as the new head of this chain.
func (store *Store) updateHead(ctx context.Context, newTS *types.TipSet) ([]*types.TipSet, []*types.TipSet, bool, error) {
	var dropped []*types.TipSet
	var added []*types.TipSet
	var err error

	if store.head != nil {
		if store.head.Equals(newTS) {
			return nil, nil, false, nil
		}
		// reorg
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
}

// SetHead sets the passed in tipset as the new head of this chain.
func (store *Store) SetHead(ctx context.Context, newTS *types.TipSet) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	return store.setHead(ctx, newTS)
}

func (store *Store) setHead(ctx context.Context, newTS *types.TipSet) error {
	log.Infof("SetHead %s %d", newTS.String(), newTS.Height())
	// Add logging to debug sporadic test failure.
	if !newTS.Defined() {
		log.Errorf("publishing empty tipset")
		log.Error(debug.Stack())
		return nil
	}

	// reorg tipset
	dropped, added, update, err := store.updateHead(ctx, newTS)
	if err != nil {
		return err
	}

	if !update {
		return nil
	}

	store.PersistTipSetKey(ctx, newTS.Key())

	// todo wrap by go function
	Reverse(added)

	// do reorg
	store.reorgCh <- reorg{
		old: dropped,
		new: added,
	}

	return nil
}

func (store *Store) PersistTipSetKey(ctx context.Context, key types.TipSetKey) {
	tskBlk, err := key.ToStorageBlock()
	if err != nil {
		log.Errorf("failed to create a block from tsk: %s", key)
	}

	_ = store.bsstore.Put(ctx, tskBlk)
}

const reorgChBuf = 32

func (store *Store) reorgWorker(ctx context.Context) chan reorg {
	headChangeNotifee := func(rev, app []*types.TipSet) error {
		notif := make([]*types.HeadChange, len(rev)+len(app))
		for i, revert := range rev {
			notif[i] = &types.HeadChange{
				Type: types.HCRevert,
				Val:  revert,
			}
		}

		for i, apply := range app {
			notif[i+len(rev)] = &types.HeadChange{
				Type: types.HCApply,
				Val:  apply,
			}
		}

		// Publish an event that we have a new head.
		store.headEvents.Pub(notif, types.HeadChangeTopic)
		return nil
	}

	out := make(chan reorg, reorgChBuf)
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
func (store *Store) SubHeadChanges(ctx context.Context) chan []*types.HeadChange {
	store.mu.RLock()
	subCh := store.headEvents.Sub(types.HeadChangeTopic)
	head := store.head
	store.mu.RUnlock()

	out := make(chan []*types.HeadChange, 16)
	out <- []*types.HeadChange{{
		Type: types.HCCurrent,
		Val:  head,
	}}

	go func() {
		defer func() {
			// Tell the caller we're done first, the following may block for a bit.
			close(out)

			// Unsubscribe.
			store.headEvents.Unsub(subCh)

			// Drain the channel.
			for range subCh {
			}
		}()

		for {
			select {
			case val, ok := <-subCh:
				if !ok {
					log.Warn("chain head sub exit loop")
					return
				}

				select {
				case out <- val.([]*types.HeadChange):
				default:
					log.Errorf("closing head change subscription due to slow reader")
					return
				}
				if len(out) > 5 {
					log.Warnf("head change sub is slow, has %d buffered entries", len(out))
				}
			case <-ctx.Done():
				log.Infof("exit sub head change: %v", ctx.Err())
				return
			}
		}
	}()
	return out
}

// SubscribeHeadChanges subscribe head change event
func (store *Store) SubscribeHeadChanges(f ReorgNotifee) {
	store.reorgNotifeeCh <- f
}

// ReadOnlyStateStore provides a read-only IPLD store for access to chain state.
func (store *Store) ReadOnlyStateStore() util.ReadOnlyIpldStore {
	return util.ReadOnlyIpldStore{IpldStore: store.stateAndBlockSource}
}

// writeHead writes the given cid set as head to disk.
func (store *Store) writeHead(ctx context.Context, tsk types.TipSetKey) error {
	log.Debugf("WriteHead %s", tsk.String())
	buf := new(bytes.Buffer)
	err := tsk.MarshalCBOR(buf)
	if err != nil {
		return err
	}

	return store.ds.Put(ctx, HeadKey, buf.Bytes())
}

// writeTipSetMetadata writes the tipset key and the state root id to the
// datastore.
func (store *Store) writeTipSetMetadata(ctx context.Context, tsm *TipSetMetadata) error {
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

	return store.ds.Put(ctx, key, buf.Bytes())
}

// deleteTipSetMetadata delete the state root id from the datastore for the tipset key.
func (store *Store) DeleteTipSetMetadata(ctx context.Context, ts *types.TipSet) error { // nolint
	store.tipIndex.Del(ts)
	h := ts.Height()
	key := datastore.NewKey(makeKey(ts.String(), h))
	return store.ds.Delete(ctx, key)
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

func recurseLinks(ctx context.Context, bs blockstore.Blockstore, walked *cid.Set, root cid.Cid, in []cid.Cid) ([]cid.Cid, error) {
	if multicodec.Code(root.Prefix().Codec) != multicodec.DagCbor {
		return in, nil
	}

	data, err := bs.Get(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("recurse links get (%s) failed: %w", root, err)
	}

	var rerr error
	err = cbg.ScanForLinks(bytes.NewReader(data.RawData()), func(c cid.Cid) {
		if rerr != nil {
			// No error return on ScanForLinks :(
			return
		}

		// traversed this already...
		if !walked.Visit(c) {
			return
		}

		in = append(in, c)
		var err error
		in, err = recurseLinks(ctx, bs, walked, c, in)
		if err != nil {
			rerr = err
		}
	})
	if err != nil {
		return nil, fmt.Errorf("scanning for links failed: %w", err)
	}

	return in, rerr
}

func (store *Store) Export(ctx context.Context, ts *types.TipSet, inclRecentRoots abi.ChainEpoch, skipOldMsgs bool, w io.Writer) error {
	h := &car.CarHeader{
		Roots:   ts.Cids(),
		Version: 1,
	}

	if err := car.WriteHeader(h, w); err != nil {
		return fmt.Errorf("failed to write car header: %s", err)
	}

	return store.WalkSnapshot(ctx, ts, inclRecentRoots, skipOldMsgs, true, func(c cid.Cid) error {
		blk, err := store.bsstore.Get(ctx, c)
		if err != nil {
			return fmt.Errorf("writing object to car, bs.Get: %w", err)
		}

		if err := carutil.LdWrite(w, c.Bytes(), blk.RawData()); err != nil {
			return fmt.Errorf("failed to write block to car output: %w", err)
		}

		return nil
	})
}

func (store *Store) WalkSnapshot(ctx context.Context, ts *types.TipSet, inclRecentRoots abi.ChainEpoch, skipOldMsgs, skipMsgReceipts bool, cb func(cid.Cid) error) error {
	if ts == nil {
		ts = store.GetHead()
	}

	seen := cid.NewSet()
	walked := cid.NewSet()

	blocksToWalk := ts.Cids()
	currentMinHeight := ts.Height()

	walkChain := func(blk cid.Cid) error {
		if !seen.Visit(blk) {
			return nil
		}

		if err := cb(blk); err != nil {
			return err
		}

		data, err := store.bsstore.Get(ctx, blk)
		if err != nil {
			return fmt.Errorf("getting block: %w", err)
		}

		var b types.BlockHeader
		if err := b.UnmarshalCBOR(bytes.NewBuffer(data.RawData())); err != nil {
			return fmt.Errorf("unmarshaling block header (cid=%s): %w", blk, err)
		}

		if currentMinHeight > b.Height {
			currentMinHeight = b.Height
			if currentMinHeight%builtin.EpochsInDay == 0 {
				log.Infow("export", "height", currentMinHeight)
			}
		}

		var cids []cid.Cid
		if !skipOldMsgs || b.Height > ts.Height()-inclRecentRoots {
			if walked.Visit(b.Messages) {
				mcids, err := recurseLinks(ctx, store.bsstore, walked, b.Messages, []cid.Cid{b.Messages})
				if err != nil {
					return fmt.Errorf("recursing messages failed: %w", err)
				}
				cids = mcids
			}
		}

		if b.Height > 0 {
			blocksToWalk = append(blocksToWalk, b.Parents...)
		} else {
			// include the genesis block
			cids = append(cids, b.Parents...)
		}

		out := cids

		if b.Height == 0 || b.Height > ts.Height()-inclRecentRoots {
			if walked.Visit(b.ParentStateRoot) {
				cids, err := recurseLinks(ctx, store.bsstore, walked, b.ParentStateRoot, []cid.Cid{b.ParentStateRoot})
				if err != nil {
					return fmt.Errorf("recursing genesis state failed: %w", err)
				}

				out = append(out, cids...)
			}

			if !skipMsgReceipts && walked.Visit(b.ParentMessageReceipts) {
				out = append(out, b.ParentMessageReceipts)
			}
		}

		for _, c := range out {
			if seen.Visit(c) {
				prefix := c.Prefix()

				// Don't include identity CIDs.
				if multicodec.Code(prefix.MhType) == multicodec.Identity {
					continue
				}

				// We only include raw, cbor, and dagcbor, for now.
				switch multicodec.Code(prefix.Codec) {
				case multicodec.Cbor, multicodec.DagCbor, multicodec.Raw:
				default:
					continue
				}

				if err := cb(c); err != nil {
					return err
				}

			}
		}

		return nil
	}

	log.Infow("export started")
	exportStart := constants.Clock.Now()

	for len(blocksToWalk) > 0 {
		next := blocksToWalk[0]
		blocksToWalk = blocksToWalk[1:]
		if err := walkChain(next); err != nil {
			return fmt.Errorf("walk chain failed: %w", err)
		}
	}

	log.Infow("export finished", "duration", constants.Clock.Now().Sub(exportStart).Seconds())

	return nil
}

// Import import a car file into local db
func (store *Store) Import(ctx context.Context, r io.Reader) (*types.TipSet, *types.BlockHeader, error) {
	br, err := carv2.NewBlockReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("loadcar failed: %w", err)
	}

	if len(br.Roots) == 0 {
		return nil, nil, fmt.Errorf("no roots in snapshot car file")
	}

	var tailBlock types.BlockHeader
	tailBlock.Height = abi.ChainEpoch(-1)
	nextTailCid := br.Roots[0]

	parallelPuts := 5
	putThrottle := make(chan error, parallelPuts)
	for i := 0; i < parallelPuts; i++ {
		putThrottle <- nil
	}

	var buf []blocks.Block
	for {
		blk, err := br.Next()
		if err != nil {
			if err == io.EOF {
				if len(buf) > 0 {
					if err := store.bsstore.PutMany(ctx, buf); err != nil {
						return nil, nil, err
					}
				}

				break
			}
			return nil, nil, err
		}

		// check for header block, looking for genesis
		if blk.Cid() == nextTailCid && tailBlock.Height != 0 {
			if err := tailBlock.UnmarshalCBOR(bytes.NewReader(blk.RawData())); err != nil {
				return nil, nil, fmt.Errorf("failed to unmarshal genesis block: %w", err)
			}
			if len(tailBlock.Parents) > 0 {
				nextTailCid = tailBlock.Parents[0]
			} else {
				// note: even the 0th block has a parent linking to the cbor genesis block
				return nil, nil, fmt.Errorf("current block (epoch %d cid %s) has no parents", tailBlock.Height, tailBlock.Cid())
			}
		}

		buf = append(buf, blk)

		if len(buf) > 1000 {
			if lastErr := <-putThrottle; lastErr != nil { // consume one error to have the right to add one
				return nil, nil, lastErr
			}

			go func(buf []blocks.Block) {
				putThrottle <- store.bsstore.PutMany(ctx, buf)
			}(buf)
			buf = nil
		}
	}

	// check errors
	for i := 0; i < parallelPuts; i++ {
		if lastErr := <-putThrottle; lastErr != nil {
			return nil, nil, lastErr
		}
	}

	if tailBlock.Height != 0 {
		return nil, nil, fmt.Errorf("expected genesis block to have height 0 (genesis), got %d: %s", tailBlock.Height, tailBlock.Cid())
	}

	root, err := store.GetTipSet(ctx, types.NewTipSetKey(br.Roots...))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load root tipset from chainfile: %w", err)
	}

	// Notice here is different with lotus, because the head tipset in lotus is not computed,
	// but in venus the head tipset is computed, so here we will fallback a pre tipset
	// and the chain store must has a metadata for each tipset, below code is to build the tipset metadata

	var (
		startHeight = root.Height()
		curTipset   = root
	)

	log.Info("import height: ", root.Height(), " root: ", root.String(), " parents: ", root.At(0).Parents)
	for {
		if curTipset.Height() <= 0 {
			break
		}
		curTipsetKey := curTipset.Parents()
		curParentTipset, err := store.GetTipSet(ctx, curTipsetKey)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load root tipset from chainfile: %w", err)
		}

		if curParentTipset.Height() == 0 {
			break
		}

		if _, err := tree.LoadState(ctx, store.stateAndBlockSource, curTipset.At(0).ParentStateRoot); err != nil {
			log.Infof("last ts height: %d, cids: %s, total import: %d", curTipset.Height(), curTipset.Key(), startHeight-curTipset.Height())
			break
		}

		// save fake root
		err = store.PutTipSetMetadata(context.Background(), &TipSetMetadata{
			TipSetStateRoot: curTipset.At(0).ParentStateRoot,
			TipSet:          curParentTipset,
			TipSetReceipts:  curTipset.At(0).ParentMessageReceipts,
		})
		if err != nil {
			return nil, nil, err
		}

		// save tipsetkey
		store.PersistTipSetKey(ctx, curParentTipset.Key())

		curTipset = curParentTipset
	}

	return root, &tailBlock, nil
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
	return store.ds.Put(ctx, CheckPoint, buf.Bytes())
}

func (store *Store) GetCirculatingSupplyDetailed(ctx context.Context, height abi.ChainEpoch, st tree.Tree) (types.CirculatingSupply, error) {
	return store.circulatingSupplyCalculator.GetCirculatingSupplyDetailed(ctx, height, st)
}

func (store *Store) GetFilVested(ctx context.Context, height abi.ChainEpoch) (abi.TokenAmount, error) {
	return store.circulatingSupplyCalculator.GetFilVested(ctx, height)
}

// StateCirculatingSupply get circulate supply at specify epoch
func (store *Store) StateCirculatingSupply(ctx context.Context, tsk types.TipSetKey) (abi.TokenAmount, error) {
	ts, err := store.GetTipSet(ctx, tsk)
	if err != nil {
		return abi.TokenAmount{}, err
	}

	root, err := store.GetTipSetStateRoot(ctx, ts)
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
		// this can be a lengthy operation, we need to cancel early when
		// the context is cancelled to avoid resource exhaustion
		select {
		case <-ctx.Done():
			// this will cause ForEach to return
			return ctx.Err()
		default:
		}

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
			a == builtin.ReserveAddress ||
			a == builtin.EthereumAddressManagerActorAddr:

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

		case builtin.IsAccountActor(actor.Code) ||
			builtin.IsPaymentChannelActor(actor.Code) ||
			builtin.IsEthAccountActor(actor.Code) ||
			builtin.IsEvmActor(actor.Code) ||
			builtin.IsPlaceholderActor(actor.Code):

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
			return fmt.Errorf("unexpected actor: %s", a)
		}

		return nil
	})
	if err != nil {
		return abi.TokenAmount{}, err
	}

	total := big.Add(circ, unCirc)
	if !total.Equals(types.TotalFilecoinInt) {
		return abi.TokenAmount{}, fmt.Errorf("total filecoin didn't add to expected amount: %s != %s", total, types.TotalFilecoinInt)
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
func (store *Store) ReorgOps(ctx context.Context, a, b *types.TipSet) ([]*types.TipSet, []*types.TipSet, error) {
	return ReorgOps(ctx, store.GetTipSet, a, b)
}

// ReorgOps takes two tipsets (which can be at different heights), and walks
// their corresponding chains backwards one step at a time until we find
// a common ancestor. It then returns the respective chain segments that fork
// from the identified ancestor, in reverse order, where the first element of
// each slice is the supplied tipset, and the last element is the common
// ancestor.
//
// If an error happens along the way, we return the error with nil slices.
// todo should move this code into store.ReorgOps. anywhere use this function should invoke store.ReorgOps
func ReorgOps(ctx context.Context,
	lts func(ctx context.Context, _ types.TipSetKey) (*types.TipSet, error),
	a, b *types.TipSet,
) ([]*types.TipSet, []*types.TipSet, error) {
	left := a
	right := b

	var leftChain, rightChain []*types.TipSet
	for !left.Equals(right) {
		// this can take a long time and lot of memory if the tipsets are far apart
		// since it can be reached through remote calls, we need to
		// cancel early when possible to prevent resource exhaustion.
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		if left.Height() > right.Height() {
			leftChain = append(leftChain, left)
			par, err := lts(ctx, left.Parents())
			if err != nil {
				return nil, nil, err
			}

			left = par
		} else {
			rightChain = append(rightChain, right)
			par, err := lts(ctx, right.Parents())
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
func (store *Store) PutMessage(ctx context.Context, m storable) (cid.Cid, error) {
	return PutMessage(ctx, store.bsstore, m)
}

// Blockstore return local blockstore
// todo remove this method, and code that need blockstore should get from blockstore submodule
func (store *Store) Blockstore() blockstoreutil.Blockstore { // nolint
	return store.bsstore
}

// GetParentReceipt get the receipt of parent tipset at specify message slot
func (store *Store) GetParentReceipt(b *types.BlockHeader, i int) (*types.MessageReceipt, error) {
	ctx := context.TODO()
	// block headers use adt0, for now.
	a, err := blockadt.AsArray(adt.WrapStore(ctx, store.stateAndBlockSource), b.ParentMessageReceipts)
	if err != nil {
		return nil, fmt.Errorf("amt load: %w", err)
	}

	var r types.MessageReceipt
	if found, err := a.Get(uint64(i), &r); err != nil {
		return nil, err
	} else if !found {
		return nil, fmt.Errorf("failed to find receipt %d", i)
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
		st, err := store.GetTipSetStateRoot(ctx, ts)
		if err != nil {
			return nil, cid.Undef, err
		}
		return ts, st, nil
	}

	// Get the tipset after the lookback tipset, or the next non-null one.
	nextTS, err := store.GetTipSetByHeight(ctx, ts, lbr+1, false)
	if err != nil {
		return nil, cid.Undef, fmt.Errorf("failed to get lookback tipset+1: %v", err)
	}

	nextTh := nextTS.Height()
	if lbr > nextTh {
		return nil, cid.Undef, fmt.Errorf("failed to find non-null tipset %s (%d) which is known to exist, found %s (%d)", ts.Key(), h, nextTS.Key(), nextTh)
	}

	pKey := nextTS.Parents()
	lbts, err := store.GetTipSet(ctx, pKey)
	if err != nil {
		return nil, cid.Undef, fmt.Errorf("failed to resolve lookback tipset: %v", err)
	}

	return lbts, nextTS.Blocks()[0].ParentStateRoot, nil
}

// Actor

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

// ResolveToDeterministicAddress get key address of specify address.
// if ths addr is bls/secpk address, return directly, other get the pubkey and generate address
func (store *Store) ResolveToDeterministicAddress(ctx context.Context, ts *types.TipSet, addr address.Address) (address.Address, error) {
	st, err := store.StateView(ctx, ts)
	if err != nil {
		return address.Undef, errors.Wrap(err, "failed to load latest state")
	}

	return st.ResolveToDeterministicAddress(ctx, addr)
}

// StateView return state view at ts epoch
func (store *Store) StateView(ctx context.Context, ts *types.TipSet) (*state.View, error) {
	if ts == nil {
		ts = store.head
	}
	root, err := store.GetTipSetStateRoot(ctx, ts)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", ts.Key().String())
	}

	return state.NewView(store.stateAndBlockSource, root), nil
}

// AccountView return account view at ts state
func (store *Store) AccountView(ctx context.Context, ts *types.TipSet) (state.AccountView, error) {
	if ts == nil {
		ts = store.head
	}
	root, err := store.GetTipSetStateRoot(ctx, ts)
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

func (store *Store) Weight(ctx context.Context, ts *types.TipSet) (big.Int, error) {
	return store.weight(ctx, store.stateAndBlockSource, ts)
}

func (store *Store) AddToTipSetTracker(ctx context.Context, b *types.BlockHeader) error {
	store.tstLk.Lock()
	defer store.tstLk.Unlock()

	tss := store.tipsets[b.Height]
	for _, oc := range tss {
		if oc == b.Cid() {
			log.Debug("tried to add block to tipset tracker that was already there")
			return nil
		}
		h, err := store.GetBlock(ctx, oc)
		if err == nil && h != nil {
			if h.Miner == b.Miner {
				log.Warnf("Have multiple blocks from miner %s at height %d in our tipset cache %s-%s", b.Miner, b.Height, b.Cid(), h.Cid())
			}
		}
	}
	// This function is called 5 times per epoch on average
	// It is also called with tipsets that are done with initial validation
	// so they cannot be from the future.
	// We are guaranteed not to use tipsets older than 900 epochs (fork limit)
	// This means that we ideally want to keep only most recent 900 epochs in here
	// Golang's map iteration starts at a random point in a map.
	// With 5 tries per epoch, and 900 entries to keep, on average we will have
	// ~136 garbage entries in the `store.tipsets` map. (solve for 1-(1-x/(900+x))^5 == 0.5)
	// Seems good enough to me

	for height := range store.tipsets {
		if height < b.Height-constants.Finality {
			delete(store.tipsets, height)
		}
		break
	}

	store.tipsets[b.Height] = append(tss, b.Cid())

	return nil
}

// RefreshHeaviestTipSet receives a newTsHeight at which a new tipset might exist. It then:
// - "refreshes" the heaviest tipset that can be formed at its current heaviest height
//   - if equivocation is detected among the miners of the current heaviest tipset, the head is immediately updated to the heaviest tipset that can be formed in a range of 5 epochs
//
// - forms the best tipset that can be formed at the _input_ height
// - compares the three tipset weights: "current" heaviest tipset, "refreshed" tipset, and best tipset at newTsHeight
// - updates "current" heaviest to the heaviest of those 3 tipsets (if an update is needed), assuming it doesn't violate the maximum fork rule
func (store *Store) RefreshHeaviestTipSet(ctx context.Context, newTSHeight abi.ChainEpoch) error {
	for {
		store.mu.Lock()
		if len(store.reorgCh) < reorgChBuf/2 {
			break
		}
		store.mu.Unlock()
		log.Errorf("reorg channel is heavily backlogged, waiting a bit before trying to take process new tipsets")
		select {
		case <-time.After(time.Second / 2):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	defer store.mu.Unlock()

	heaviest := store.head
	heaviestWeight, err := store.Weight(ctx, heaviest)
	if err != nil {
		return fmt.Errorf("failed to calculate currentHeaviest's weight: %w", err)
	}

	heaviestHeight := abi.ChainEpoch(0)
	if heaviest != nil {
		heaviestHeight = heaviest.Height()
	}

	// Before we look at newTs, let's refresh best tipset at current head's height -- this is done to detect equivocation
	newHeaviest, newHeaviestWeight, err := store.FormHeaviestTipSetForHeight(ctx, heaviestHeight)
	if err != nil {
		return fmt.Errorf("failed to reform head at same height: %w", err)
	}

	// Equivocation has occurred! We need a new head NOW!
	if newHeaviest == nil || newHeaviestWeight.LessThan(heaviestWeight) {
		log.Warnf("chainstore heaviest tipset's weight SHRANK from %d (%s) to %d (%s) due to equivocation", heaviestWeight, heaviest, newHeaviestWeight, newHeaviest)
		// Unfortunately, we don't know what the right height to form a new heaviest tipset is.
		// It is _probably_, but not _necessarily_, heaviestHeight.
		// So, we need to explore a range of epochs, finding the heaviest tipset in that range.
		// We thus try to form the heaviest tipset for 5 epochs above heaviestHeight (most of which will likely not exist),
		// as well as for 5 below.
		// This is slow, but we expect to almost-never be here (only if miners are equivocating, which carries a hefty penalty).
		for i := heaviestHeight + 5; i > heaviestHeight-5; i-- {
			possibleHeaviestTS, possibleHeaviestWeight, err := store.FormHeaviestTipSetForHeight(ctx, i)
			if err != nil {
				return fmt.Errorf("failed to produce head at height %d: %w", i, err)
			}

			if possibleHeaviestWeight.GreaterThan(newHeaviestWeight) {
				newHeaviestWeight = possibleHeaviestWeight
				newHeaviest = possibleHeaviestTS
			}
		}

		// if we've found something, we know it's the heaviest equivocation-free head, take it IMMEDIATELY
		if newHeaviest != nil {
			errTake := store.setHead(ctx, newHeaviest)
			if errTake != nil {
				return fmt.Errorf("failed to take newHeaviest tipset as head: %w", err)
			}
		} else {
			// if we haven't found something, just stay with our equivocation-y head
			newHeaviest = heaviest
		}
	}

	// if the new height we were notified about isn't what we just refreshed at, see if we have a heavier tipset there
	if newTSHeight != newHeaviest.Height() {
		bestTS, bestTSWeight, err := store.FormHeaviestTipSetForHeight(ctx, newTSHeight)
		if err != nil {
			return fmt.Errorf("failed to form new heaviest tipset at height %d: %w", newTSHeight, err)
		}

		heavier := bestTSWeight.GreaterThan(newHeaviestWeight)
		if bestTSWeight.Equals(newHeaviestWeight) {
			heavier = breakWeightTie(bestTS, newHeaviest)
		}

		if heavier {
			newHeaviest = bestTS
		}
	}

	// Everything's the same as before, exit early
	if newHeaviest.Equals(heaviest) {
		return nil
	}

	// At this point, it MUST be true that newHeaviest is heavier than store.heaviest -- update if fork allows
	exceeds, err := store.exceedsForkLength(ctx, heaviest, newHeaviest)
	if err != nil {
		return fmt.Errorf("failed to check fork length: %w", err)
	}

	if exceeds {
		return nil
	}

	err = store.setHead(ctx, newHeaviest)
	if err != nil {
		return fmt.Errorf("failed to take heaviest tipset: %w", err)
	}

	return nil
}

// FormHeaviestTipSetForHeight looks up all valid blocks at a given height, and returns the heaviest tipset that can be made at that height
// It does not consider ANY blocks from miners that have "equivocated" (produced 2 blocks at the same height)
func (store *Store) FormHeaviestTipSetForHeight(ctx context.Context, height abi.ChainEpoch) (*types.TipSet, types.BigInt, error) {
	store.tstLk.Lock()
	defer store.tstLk.Unlock()

	blockCids, ok := store.tipsets[height]
	if !ok {
		return nil, types.NewInt(0), nil
	}

	// First, identify "bad" miners for the height

	seenMiners := map[address.Address]struct{}{}
	badMiners := map[address.Address]struct{}{}
	blocks := make([]*types.BlockHeader, 0, len(blockCids))
	for _, bhc := range blockCids {
		h, err := store.GetBlock(ctx, bhc)
		if err != nil {
			return nil, types.NewInt(0), fmt.Errorf("failed to load block (%s) for tipset expansion: %w", bhc, err)
		}

		if _, seen := seenMiners[h.Miner]; seen {
			badMiners[h.Miner] = struct{}{}
			continue
		}
		seenMiners[h.Miner] = struct{}{}
		blocks = append(blocks, h)
	}

	// Next, group by parent tipset

	formableTipsets := make(map[types.TipSetKey][]*types.BlockHeader, 0)
	for _, h := range blocks {
		if _, bad := badMiners[h.Miner]; bad {
			continue
		}
		ptsk := types.NewTipSetKey(h.Parents...)
		formableTipsets[ptsk] = append(formableTipsets[ptsk], h)
	}

	maxWeight := types.NewInt(0)
	var maxTS *types.TipSet
	for _, headers := range formableTipsets {
		ts, err := types.NewTipSet(headers)
		if err != nil {
			return nil, types.NewInt(0), fmt.Errorf("unexpected error forming tipset: %w", err)
		}

		weight, err := store.Weight(ctx, ts)
		if err != nil {
			return nil, types.NewInt(0), fmt.Errorf("failed to calculate weight: %w", err)
		}

		heavier := weight.GreaterThan(maxWeight)
		if weight.Equals(maxWeight) {
			heavier = breakWeightTie(ts, maxTS)
		}

		if heavier {
			maxWeight = weight
			maxTS = ts
		}
	}

	return maxTS, maxWeight, nil
}

func breakWeightTie(ts1, ts2 *types.TipSet) bool {
	s := len(ts1.Blocks())
	if s > len(ts2.Blocks()) {
		s = len(ts2.Blocks())
	}

	// blocks are already sorted by ticket
	for i := 0; i < s; i++ {
		if ts1.Blocks()[i].Ticket.Less(ts2.Blocks()[i].Ticket) {
			log.Infof("weight tie broken in favour of %s", ts1.Key())
			return true
		}
	}

	log.Infof("weight tie left unbroken, default to %s", ts2.Key())
	return false
}

// Check if the two tipsets have a fork length above `ForkLengthThreshold`.
// `synced` is the head of the chain we are currently synced to and `external`
// is the incoming tipset potentially belonging to a forked chain. It assumes
// the external chain has already been validated and available in the ChainStore.
// The "fast forward" case is covered in this logic as a valid fork of length 0.
//
// FIXME: We may want to replace some of the logic in `syncFork()` with this.
//
//	`syncFork()` counts the length on both sides of the fork at the moment (we
//	need to settle on that) but here we just enforce it on the `synced` side.
func (store *Store) exceedsForkLength(ctx context.Context, synced, external *types.TipSet) (bool, error) {
	if synced == nil || external == nil {
		// FIXME: If `cs.heaviest` is nil we should just bypass the entire
		//  `MaybeTakeHeavierTipSet` logic (instead of each of the called
		//  functions having to handle the nil case on their own).
		return false, nil
	}

	var err error
	// `forkLength`: number of tipsets we need to walk back from the our `synced`
	// chain to the common ancestor with the new `external` head in order to
	// adopt the fork.
	for forkLength := 0; forkLength < int(constants.ForkLengthThreshold); forkLength++ {
		// First walk back as many tipsets in the external chain to match the
		// `synced` height to compare them. If we go past the `synced` height
		// the subsequent match will fail but it will still be useful to get
		// closer to the `synced` head parent's height in the next loop.
		for external.Height() > synced.Height() {
			if external.Height() == 0 {
				// We reached the genesis of the external chain without a match;
				// this is considered a fork outside the allowed limit (of "infinite"
				// length).
				return true, nil
			}

			external, err = store.GetTipSet(ctx, external.Parents())
			if err != nil {
				return false, fmt.Errorf("failed to load parent tipset in external chain: %w", err)
			}
		}

		// Now check if we arrived at the common ancestor.
		if synced.Equals(external) {
			return false, nil
		}

		// Now check to see if we've walked back to the checkpoint.
		if synced.Key().Equals(store.checkPoint) {
			return true, nil
		}

		// If we didn't, go back *one* tipset on the `synced` side (incrementing
		// the `forkLength`).
		if synced.Height() == 0 {
			// Same check as the `external` side, if we reach the start (genesis)
			// there is no common ancestor.
			return true, nil
		}
		synced, err = store.GetTipSet(ctx, synced.Parents())
		if err != nil {
			return false, fmt.Errorf("failed to load parent tipset in synced chain: %w", err)
		}
	}

	// We traversed the fork length allowed without finding a common ancestor.
	return true, nil
}
