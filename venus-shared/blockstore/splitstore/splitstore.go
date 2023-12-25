package splitstore

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"

	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/ipfs/boxo/blockstore"
)

const (
	CHECKPOINT_DURATION = 4 * policy.ChainFinality
)

type Closer interface {
	Close() error
}

type Cleaner interface {
	Clean() error
}
type CleanFunc func() error

func (c CleanFunc) Clean() error {
	return c()
}

var _ Cleaner = (*CleanFunc)(nil)

type BaseKeeper interface {
	Base() cid.Cid
}
type BaseFunc func() cid.Cid

func (b BaseFunc) Base() cid.Cid {
	return b()
}

var _ BaseKeeper = (*BaseFunc)(nil)

type Layer interface {
	blockstore.Blockstore
	Cleaner
	BaseKeeper
}
type LayerImpl struct {
	blockstore.Blockstore
	Cleaner
	BaseKeeper
}

type Option struct {
	MaxLayerCount   int
	LayerSize       abi.ChainEpoch
	InitSyncProtect abi.ChainEpoch
	HardDelete      bool
}

type Controller interface {
	Rollback() error
}

type Splitstore struct {
	path            string
	maxLayerCount   int
	layerSize       abi.ChainEpoch
	initSyncProtect abi.ChainEpoch

	initStore blockstore.Blockstore
	layers    *SafeSlice[Layer]

	epochToAppend abi.ChainEpoch
	epochToDrop   abi.ChainEpoch

	headTipsetKey cid.Cid
	isRollback    bool

	checkPoint cid.Cid

	taskQue chan func()
}

var _ blockstore.Blockstore = (*Splitstore)(nil)
var _ Controller = (*Splitstore)(nil)

func NewSplitstore(path string, initStore blockstore.Blockstore, opts ...Option) (*Splitstore, error) {
	opt := Option{
		MaxLayerCount:   3,
		LayerSize:       3 * policy.ChainFinality,
		InitSyncProtect: 3 * builtin.EpochsInDay,
	}
	if len(opts) > 1 {
		return nil, fmt.Errorf("splitstore: too many options")
	}
	if len(opts) == 1 {
		if opts[0].MaxLayerCount > 1 {
			opt.MaxLayerCount = opts[0].MaxLayerCount
		} else {
			log.Warnf("splitstore: max layer count must greater than 1, use default value %d", opt.MaxLayerCount)
		}

		if opts[0].LayerSize > policy.ChainFinality {
			opt.LayerSize = opts[0].LayerSize
		} else {
			log.Warnf("splitstore: layer size must greater than chain finality, use default value %d", opt.LayerSize)
		}

		opt.LayerSize = opts[0].LayerSize

		if opts[0].InitSyncProtect > 0 {
			opt.InitSyncProtect = opts[0].InitSyncProtect
		}
	}

	// check path
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(path, 0755)
			if err != nil {
				return nil, err
			}
		}
	}
	if stat != nil && !stat.IsDir() {
		return nil, fmt.Errorf("split store path %s is not a directory", path)
	}

	ss := &Splitstore{
		path:   path,
		layers: NewSafeSlice([]Layer{}),

		// must greater than 1
		maxLayerCount: opt.MaxLayerCount,
		layerSize:     opt.LayerSize,

		epochToAppend: 0,
		epochToDrop:   math.MaxInt64,

		initStore:       initStore,
		initSyncProtect: opt.InitSyncProtect,

		taskQue: make(chan func(), 50),
	}

	// scan for stores
	bs, err := scan(path)
	if err != nil {
		return nil, err
	}

	for i := range bs {
		ss.layers.Append(bs[i])
	}

	// update epochToAppend
	if ss.layers.Len() > 0 {
		ctx := context.Background()
		tskCid := ss.layers.Last().Base()
		var tsk types.TipSetKey
		err = ss.getCbor(ctx, tskCid, &tsk)
		if err != nil {
			return nil, fmt.Errorf("load store base tsk(%s): %w", tskCid, err)
		}
		ts, err := ss.getTipset(ctx, &tsk)
		if err != nil {
			return nil, fmt.Errorf("load store base tipset(%s): %w", tsk, err)
		}
		ss.epochToAppend = ts.Height() + ss.layerSize
	}

	ss.dropRedundantLayer()

	ss.start()
	log.Infof("load splitstore with %d layer", ss.layers.Len())
	return ss, nil
}

// HeadChange subscribe to head change, schedule for new store and delete old
func (ss *Splitstore) HeadChange(_, apply []*types.TipSet) error {
	if ss.isRollback {
		return nil
	}

	ctx := context.Background()

	for _, ts := range apply {
		height := ts.Height()
		log := log.With("height", height)
		var err error
		tsk := ts.Key()
		tskCid, err := tsk.Cid()
		if err != nil {
			log.Errorf("get tipset(%d) key: %s", height, err)
			continue
		}
		ss.headTipsetKey = tskCid

		if height >= ss.epochToDrop && ss.layers.Len() > ss.maxLayerCount {
			ss.epochToDrop = height + ss.layerSize
			ss.dropRedundantLayer()
		}

		if height >= ss.epochToAppend {
			ss.epochToAppend = height + ss.layerSize
			if ss.epochToDrop == math.MaxInt64 {
				ss.epochToDrop = height + ss.layerSize/2 + ss.initSyncProtect
			}

			// make sure tsk have been persisted
			h, err := ss.Has(ctx, tskCid)
			if err != nil {
				log.Warnf("check tsk(%s) exist: %s", tskCid, err)
			}
			if !h {
				tskBlock, err := tsk.ToStorageBlock()
				if err != nil {
					log.Errorf("tsk(%s) to storage block: %s", tskCid, err)
				}
				err = ss.Put(ctx, tskBlock)
				if err != nil {
					log.Errorf("persist tsk(%s): %s", tskCid, err)
				}
			}

			// snapshot store before append
			snapStore := ss.composeStore()

			layer, err := newLayer(ss.path, int64(height), tskCid)
			if err != nil {
				log.Errorf("create new layer: %s", err)
				continue
			}
			ss.layers.Append(layer)
			log.Infof("append new layer base(%s)", tskCid.String())

			// backup header to init store
			err = backupHeader(ctx, tskCid, NewComposeStore(snapStore, ss.initStore), ss.initStore)
			if err != nil {
				log.Errorf("append new layer: backup header: %v", err)
			}

			// backup state

		}
	}
	return nil
}

// AllKeysChan implements blockstore.Blockstore.
func (ss *Splitstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return ss.composeStore().AllKeysChan(ctx)
}

// DeleteBlock implements blockstore.Blockstore.
func (ss *Splitstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return ss.composeStore().DeleteBlock(ctx, c)
}

// DeleteMany implements blockstore.Blockstore.
func (ss *Splitstore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	return ss.composeStore().DeleteMany(ctx, cids)
}

// Flush implements blockstore.Blockstore.
func (ss *Splitstore) Flush(ctx context.Context) error {
	return ss.composeStore().Flush(ctx)
}

// Get implements blockstore.Blockstore.
func (ss *Splitstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	return ss.composeStore().Get(ctx, c)
}

// GetSize implements blockstore.Blockstore.
func (ss *Splitstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	return ss.composeStore().GetSize(ctx, c)
}

// Has implements blockstore.Blockstore.
func (ss *Splitstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return ss.composeStore().Has(ctx, c)
}

// HashOnRead implements blockstore.Blockstore.
func (ss *Splitstore) HashOnRead(enabled bool) {
	ss.composeStore().HashOnRead(enabled)
}

// Put implements blockstore.Blockstore.
func (ss *Splitstore) Put(ctx context.Context, b blocks.Block) error {
	return ss.composeStore().Put(ctx, b)
}

// PutMany implements blockstore.Blockstore.
func (ss *Splitstore) PutMany(ctx context.Context, bs []blocks.Block) error {
	return ss.composeStore().PutMany(ctx, bs)
}

// View implements blockstore.Blockstore.
func (ss *Splitstore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	return ss.composeStore().View(ctx, cid, callback)
}

// Close close all sub store
func (ss *Splitstore) Close() error {
	ss.layers.ForEach(func(i int, store Layer) bool {
		if closer, ok := store.(Closer); ok {
			err := closer.Close()
			if err != nil {
				log.Errorf("close %dth store: %s", i, err)
			}
		}
		return true
	})

	if ss.isRollback {
		// try best to clean all store
		ss.layers.ForEach(func(i int, store Layer) bool {
			err := store.Clean()
			if err != nil {
				bsCid := store.Base()
				log.Errorf("clean store(%s) fail, try to clean manually: %w", bsCid, err)
			}
			return true
		})
	}
	return nil
}

// Rollback rollback splitstore to init store:
// 1. redirect query to init store
// 2. disable store appending and dropping
// 3. transfer blocks back to init store
// 4. clean all stores
func (ss *Splitstore) Rollback() error {
	ss.isRollback = true
	ctx := context.Background()

	if !ss.headTipsetKey.Defined() {
		return fmt.Errorf("rollback: head tipset key is undefined")
	}

	// transfer state to init store
	currentHeight, err := ss.getTipsetHeight(ctx, ss.headTipsetKey)
	if err != nil {
		return fmt.Errorf("get current height: %w", err)
	}
	targetHeight := currentHeight - CHECKPOINT_DURATION
	err = WalkUntil(ctx, ss.composeStore(), ss.headTipsetKey, targetHeight)
	if err != nil {
		return fmt.Errorf("transfer state to init store: %w", err)
	}

	// transfer block header to init store
	backupHeader(ctx, ss.headTipsetKey, ss.composeStore(), ss.initStore)

	return nil
}

// Len return the number of store layers
func (ss *Splitstore) Len() int {
	return ss.layers.Len()
}

func (ss *Splitstore) dropRedundantLayer() {
	if ss.layers.Len() <= ss.maxLayerCount {
		return
	}

	// collect state, then close and clean
	ss.taskQue <- func() {
		if !(ss.layers.Len() > ss.maxLayerCount) {
			log.Warnf("drop last layer: unexpected layer count(%d)", ss.layers.Len())
			return
		}

		dropCount := ss.layers.Len() - ss.maxLayerCount
		targetStore := ss.layers.At(ss.layers.Len() - 2)
		bs := []blockstore.Blockstore{ss.initStore}
		ss.layers.Slice(0, ss.layers.Len()-1).ForEach(func(i int, store Layer) bool {
			bs = append(bs, store)
			return true
		})
		snapshot := NewComposeStore(bs...)

		if !ss.checkPoint.Equals(targetStore.Base()) {
			var height abi.ChainEpoch
			height, err := ss.getTipsetHeight(context.Background(), targetStore.Base())
			if err != nil {
				log.Errorf("collect state: get tipset height: %s", err)
			}

			// collect the last state of target store
			err = WalkUntil(context.Background(), snapshot, targetStore.Base(), height-CHECKPOINT_DURATION)
			ss.checkPoint = targetStore.Base()
		}

		for i := 0; i < dropCount; i++ {
			storeToDrop := ss.layers.First()
			// close and clean
			if closer, ok := storeToDrop.(Closer); ok {
				err := closer.Close()
				if err != nil {
					log.Errorf("close store(%s): %s", storeToDrop.Base(), err)
				}
			}
			err := storeToDrop.Clean()
			if err != nil {
				log.Errorf("clean store(%s): %s", storeToDrop.Base(), err)
			}
			ss.layers.Delete(0)
		}
	}
}

func (ss *Splitstore) composeStore() blockstore.Blockstore {
	bs := make([]blockstore.Blockstore, 0, ss.layers.Len())
	ss.layers.ForEach(func(i int, store Layer) bool {
		bs = append(bs, store)
		return true
	})

	if ss.isRollback {
		bs = append(bs, ss.initStore)
	} else {
		bs = append([]blockstore.Blockstore{ss.initStore}, bs...)
	}

	return NewComposeStore(bs...)
}

func (ss *Splitstore) start() {
	go func() {
		for {
			select {
			case task := <-ss.taskQue:
				task()
			}
		}
	}()
}

func (ss *Splitstore) getCbor(ctx context.Context, c cid.Cid, out interface{}) error {
	cst := cbor.NewCborStore(ss.composeStore())
	return cst.Get(ctx, c, out)
}

func (ss *Splitstore) getTipsetKey(ctx context.Context, c cid.Cid) (*types.TipSetKey, error) {
	var tsk types.TipSetKey
	err := ss.getCbor(ctx, c, &tsk)
	if err != nil {
		return nil, err
	}
	return &tsk, nil
}

func (ss *Splitstore) getTipset(ctx context.Context, key *types.TipSetKey) (*types.TipSet, error) {
	if key.IsEmpty() {
		return nil, fmt.Errorf("get tipset: tipset key is empty")
	}

	cids := key.Cids()
	blks := make([]*types.BlockHeader, len(cids))
	for idx, c := range cids {
		var blk types.BlockHeader
		err := ss.getCbor(ctx, c, &blk)
		if err != nil {
			return nil, err
		}

		blks[idx] = &blk
	}

	ts, err := types.NewTipSet(blks)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func (ss *Splitstore) getBlock(ctx context.Context, c cid.Cid) (*types.BlockHeader, error) {
	if !c.Defined() {
		return nil, fmt.Errorf("get block: block cid is undefined")
	}

	var blk types.BlockHeader
	err := ss.getCbor(ctx, c, &blk)
	if err != nil {
		return nil, err
	}

	return &blk, nil
}

func (ss *Splitstore) getTipsetHeight(ctx context.Context, key cid.Cid) (abi.ChainEpoch, error) {
	if !key.Defined() {
		return 0, fmt.Errorf("get tipset: tipset key is undefined")
	}

	var tsk types.TipSetKey
	if err := ss.getCbor(ctx, key, &tsk); err != nil {
		return 0, err
	}

	if tsk.IsEmpty() {
		return 0, fmt.Errorf("get tipset: tipset key is empty")
	}

	cids := tsk.Cids()
	var blk types.BlockHeader
	if err := ss.getCbor(ctx, cids[0], &blk); err != nil {
		return 0, err
	}

	return blk.Height, nil
}

// scan for stores from splitstore path
func scan(path string) ([]*LayerImpl, error) {
	// path have been check before
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var bs []*LayerImpl
	heights := make(map[*LayerImpl]int64)

	// name of entry need to match '%d_%s' pattern
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		height, c, err := extractHeightAndCid(name)
		if err != nil {
			continue
		}

		innerStore, err := newLayer(path, height, c)
		if err != nil {
			return nil, fmt.Errorf("scan splitstore: %w", err)
		}

		bs = append(bs, innerStore)
		heights[innerStore] = (height)
	}

	// sort by height ASC
	sort.Slice(bs, func(i, j int) bool {
		return heights[bs[i]] < heights[bs[j]]
	})

	return bs, nil
}

func newLayer(path string, height int64, c cid.Cid) (*LayerImpl, error) {
	if !c.Defined() {
		return nil, fmt.Errorf("new inner store: cid is undefined")
	}

	var store blockstore.Blockstore
	storePath := filepath.Join(path, fmt.Sprintf("base_%d_%s.db", height, c))

	stat, err := os.Stat(storePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(storePath, 0755)
		if err != nil {
			return nil, fmt.Errorf("create store path(%s): %w", storePath, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("check store path(%s): %w", storePath, err)
	}

	if stat != nil && !stat.IsDir() {
		return nil, fmt.Errorf("store path(%s) is not a directory", storePath)
	}

	opt, _ := blockstore.BadgerBlockstoreOptions(storePath, false)
	opt.Prefix = bstore.BlockPrefix.String()
	store, err = blockstore.Open(opt)
	if err != nil {
		return nil, err
	}

	return &LayerImpl{
		Blockstore: store,
		Cleaner: CleanFunc(func() error {
			// fake delete: just rename with suffix '.del'
			return os.Rename(storePath, storePath+".del")

			// return os.RemoveAll(storePath)
		}),
		BaseKeeper: BaseFunc(func() cid.Cid {
			return c
		}),
	}, nil
}

func extractHeightAndCid(s string) (int64, cid.Cid, error) {
	re := regexp.MustCompile(`base_(\d+)_(\w+)\.db$`)
	match := re.FindStringSubmatch(s)
	if len(match) != 3 {
		return 0, cid.Undef, fmt.Errorf("failed to extract height and cid from %s", s)
	}
	height, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, cid.Undef, err
	}
	if match[2] == "init" {
		return height, cid.Undef, nil
	}
	c, err := cid.Parse(match[2])
	if err != nil {
		return 0, cid.Undef, err
	}
	return height, c, nil
}
