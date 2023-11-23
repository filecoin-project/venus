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
	"sync"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/venus-shared/actors/policy"
	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"

	cbor "github.com/ipfs/go-ipld-cbor"

	bstore "github.com/ipfs/boxo/blockstore"
)

type Closer interface {
	Close() error
}

type Cleaner interface {
	Clean() error
}
type CleanImpl func() error

func (c CleanImpl) Clean() error {
	return c()
}

var _ Cleaner = (*CleanImpl)(nil)

type BaseKeeper interface {
	Base() cid.Cid
}
type BaseKeeperImpl func() cid.Cid

func (b BaseKeeperImpl) Base() cid.Cid {
	return b()
}

var _ BaseKeeper = (*BaseKeeperImpl)(nil)

type InnerBlockstore interface {
	blockstore.Blockstore
	Cleaner
	BaseKeeper
}
type InnerBlockstoreImpl struct {
	blockstore.Blockstore
	Cleaner
	BaseKeeper
}

type Option struct {
	MaxStoreCount int
	StoreSize     abi.ChainEpoch
}

type Controller interface {
	Rollback() error
}

type Splitstore struct {
	path          string
	maxStoreCount int
	storeSize     abi.ChainEpoch
	stores        []InnerBlockstore

	epochToAppend abi.ChainEpoch
	epochToClean  abi.ChainEpoch

	initStore     blockstore.Blockstore
	headTipsetKey cid.Cid
	isRollback    bool

	mut sync.RWMutex
}

var _ blockstore.Blockstore = (*Splitstore)(nil)
var _ Controller = (*Splitstore)(nil)

func NewSplitstore(path string, initStore blockstore.Blockstore, opts ...Option) (*Splitstore, error) {
	opt := Option{
		MaxStoreCount: 3,
		StoreSize:     3 * policy.ChainFinality,
	}
	if len(opts) > 1 {
		return nil, fmt.Errorf("splitstore: too many options")
	}
	if len(opts) == 1 {
		if opts[0].MaxStoreCount > 1 {
			opt.MaxStoreCount = opts[0].MaxStoreCount
		} else {
			log.Warnf("splitstore: max store count must greater than 1, use default value %d", opt.MaxStoreCount)
		}

		if opts[0].StoreSize > policy.ChainFinality {
			opt.StoreSize = opts[0].StoreSize
		} else {
			log.Warnf("splitstore: store size must greater than chain finality, use default value %d", opt.StoreSize)
		}

		opt.StoreSize = opts[0].StoreSize
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
		stores: []InnerBlockstore{},

		// must greater than 1
		maxStoreCount: opt.MaxStoreCount,
		storeSize:     opt.StoreSize,

		epochToAppend: 0,
		epochToClean:  math.MaxInt64,

		initStore: initStore,
	}

	// scan for stores
	bs, err := scan(path)
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		// init a placeholder and first store
		innerStore, err := newInnerStore(path, 0, cid.Undef)
		if err != nil {
			return nil, err
		}
		bs = append(bs, innerStore)
	}

	if !bs[0].Base().Defined() {
		bs[0].Blockstore = initStore
	}

	for i := range bs {
		ss.stores = append(ss.stores, bs[i])
	}

	// update epochToAppend and epochToClean
	if len(ss.stores) > 1 {
		ctx := context.Background()
		tskCid := ss.stores[len(ss.stores)-1].Base()
		var tsk types.TipSetKey
		err = ss.getCbor(ctx, tskCid, &tsk)
		if err != nil {
			return nil, fmt.Errorf("load store base tsk(%s): %w", tskCid, err)
		}
		ts, err := ss.getTipset(ctx, tsk)
		if err != nil {
			return nil, fmt.Errorf("load store base tipset(%s): %w", tsk, err)
		}
		ss.epochToAppend = ts.Height() + ss.storeSize
	}

	for i := 0; i < len(ss.stores)-ss.maxStoreCount; i++ {
		err := ss.dropLastStore()
		if err != nil {
			return nil, fmt.Errorf("drop redundant store: %w", err)
		}
	}

	return ss, nil
}

// HeadChange subscribe to head change, schedule for new store and delete old
func (ss *Splitstore) HeadChange(_, apply []*types.TipSet) error {
	if ss.isRollback {
		return nil
	}

	for _, ts := range apply {
		height := ts.Height()
		log := log.With("height", height)
		var err error
		ss.headTipsetKey, err = ts.Key().Cid()
		if err != nil {
			log.Errorf("update head tipset key: %w", err)
		}

		if height >= ss.epochToClean && len(ss.stores) > ss.maxStoreCount {
			err := ss.dropLastStore()
			if err != nil {
				return err
			}
		}

		if height >= ss.epochToAppend {
			ss.epochToAppend = height + ss.storeSize
			cleanDelay := ss.storeSize / 2
			ss.epochToClean = height + cleanDelay
			tsk := ts.Key()
			tskCid, err := tsk.Cid()
			if err != nil {
				return err
			}

			// make sure tsk have been persisted
			h, err := ss.Has(context.Background(), tskCid)
			if err != nil {
				return fmt.Errorf("check tsk(%s) exist: %w", tskCid, err)
			}
			if !h {
				tskBlock, err := tsk.ToStorageBlock()
				if err != nil {
					return err
				}
				err = ss.Put(context.Background(), tskBlock)
				if err != nil {
					return fmt.Errorf("persist tsk(%s): %w", tskCid, err)
				}
			}

			store, err := newInnerStore(ss.path, int64(height), tskCid)
			if err != nil {
				return err
			}
			ss.stores = append(ss.stores, store)

			log.Infof("append new store base(%s)", tskCid.String())
		}
	}
	return nil
}

// AllKeysChan implements blockstore.Blockstore.
func (ss *Splitstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().AllKeysChan(ctx)
}

// DeleteBlock implements blockstore.Blockstore.
func (ss *Splitstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().DeleteBlock(ctx, c)
}

// DeleteMany implements blockstore.Blockstore.
func (ss *Splitstore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().DeleteMany(ctx, cids)
}

// Flush implements blockstore.Blockstore.
func (ss *Splitstore) Flush(ctx context.Context) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().Flush(ctx)
}

// Get implements blockstore.Blockstore.
func (ss *Splitstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().Get(ctx, c)
}

// GetSize implements blockstore.Blockstore.
func (ss *Splitstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().GetSize(ctx, c)
}

// Has implements blockstore.Blockstore.
func (ss *Splitstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().Has(ctx, c)
}

// HashOnRead implements blockstore.Blockstore.
func (ss *Splitstore) HashOnRead(enabled bool) {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	ss.composeStore().HashOnRead(enabled)
}

// Put implements blockstore.Blockstore.
func (ss *Splitstore) Put(ctx context.Context, b blocks.Block) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().Put(ctx, b)
}

// PutMany implements blockstore.Blockstore.
func (ss *Splitstore) PutMany(ctx context.Context, bs []blocks.Block) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().PutMany(ctx, bs)
}

// View implements blockstore.Blockstore.
func (ss *Splitstore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	ss.mut.RLock()
	defer ss.mut.RUnlock()
	return ss.composeStore().View(ctx, cid, callback)
}

// Close close all sub store
func (ss *Splitstore) Close() error {
	ss.mut.Lock()
	defer ss.mut.Unlock()
	for i := range ss.stores {
		if closer, ok := ss.stores[i].(Closer); ok {
			err := closer.Close()
			if err != nil {
				return fmt.Errorf("close %dth store: %w", i, err)
			}
		}
	}
	if ss.isRollback {
		// try best to clean all store
		for i := range ss.stores {
			err := ss.stores[i].Clean()
			if err != nil {
				bsCid := ss.stores[i].Base()
				log.Errorf("clean store(%s) fail, try to clean manually: %w", bsCid, err)
			}
		}
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

	// transfer block to init store
	if ss.headTipsetKey.Defined() {
		v := NewSyncVisitor()
		err := WalkChain(context.Background(), ss.composeStore(), ss.headTipsetKey, v, ss.storeSize)
		if err != nil {
			return fmt.Errorf("transfer block to init store: %w", err)
		}
	}

	// clean
	ss.mut.Lock()
	defer ss.mut.Unlock()
	for i := range ss.stores {
		bsCid := ss.stores[i].Base()
		if bsCid.Defined() {
			if closer, ok := ss.stores[i].(Closer); ok {
				err := closer.Close()
				if err != nil {
					return fmt.Errorf("close store(%s): %w", bsCid, err)
				}
			}
			err := ss.stores[i].Clean()
			if err != nil {
				return fmt.Errorf("clean store(%s): %w", bsCid, err)
			}
		}
	}
	ss.stores = []InnerBlockstore{}

	return nil
}

func (ss *Splitstore) dropLastStore() error {
	if len(ss.stores) < 3 {
		return fmt.Errorf("unexpected store count when launch a drop task: %d", len(ss.stores))
	}
	storeToDrop := ss.stores[0]
	log.Infof("drop store base(%s)", storeToDrop.Base())

	// block transfer
	tempStore := NewComposeStore(storeToDrop, ss.stores[1])
	v := NewSyncVisitor()
	err := WalkChain(context.Background(), tempStore, ss.stores[2].Base(), v, ss.storeSize)
	if err != nil {
		return fmt.Errorf("block transfer: %w", err)
	}

	ss.stores = ss.stores[1:]

	// close and  clean
	ss.mut.Lock()
	defer ss.mut.Unlock()
	if closer, ok := storeToDrop.(Closer); ok {
		err := closer.Close()
		if err != nil {
			return fmt.Errorf("close deprecated store: %w", err)
		}
	}
	err = storeToDrop.Clean()
	if err != nil {
		return fmt.Errorf("clean deprecated store: %w", err)
	}

	return nil
}

func (ss *Splitstore) composeStore() blockstore.Blockstore {
	bs := make([]blockstore.Blockstore, len(ss.stores))
	for i := range ss.stores {
		bs[i] = ss.stores[i]
	}
	if ss.isRollback {
		bs = append(bs, ss.initStore)
	}
	return NewComposeStore(bs...)
}

func (ss *Splitstore) getCbor(ctx context.Context, c cid.Cid, out interface{}) error {
	cst := cbor.NewCborStore(ss.composeStore())
	return cst.Get(ctx, c, out)
}

func (ss *Splitstore) getTipset(ctx context.Context, key types.TipSetKey) (*types.TipSet, error) {
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

// scan for stores from splitstore path
func scan(path string) ([]*InnerBlockstoreImpl, error) {
	// path have been check before
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var bs []*InnerBlockstoreImpl
	heights := make(map[*InnerBlockstoreImpl]int64)

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

		innerStore, err := newInnerStore(path, height, c)
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

func newInnerStore(path string, height int64, c cid.Cid) (*InnerBlockstoreImpl, error) {
	var store blockstore.Blockstore
	storePath := filepath.Join(path, fmt.Sprintf("base_%d_%s.db", height, c))
	if !c.Defined() {
		storePath = filepath.Join(path, "base_init.db")
	}

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

	if c.Defined() {
		opt, _ := blockstore.BadgerBlockstoreOptions(storePath, false)
		opt.Prefix = bstore.BlockPrefix.String()
		var err error
		store, err = blockstore.Open(opt)
		if err != nil {
			return nil, err
		}
	}

	return &InnerBlockstoreImpl{
		Blockstore: store,
		Cleaner: CleanImpl(func() error {
			return os.RemoveAll(storePath)
		}),
		BaseKeeper: BaseKeeperImpl(func() cid.Cid {
			return c
		}),
	}, nil
}

func extractHeightAndCid(s string) (int64, cid.Cid, error) {
	if s == "base_init.db" {
		return -1, cid.Undef, nil
	}
	re := regexp.MustCompile(`base_(\d+)_(\w+)\.db`)
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
