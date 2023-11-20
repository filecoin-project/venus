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

type SplitstoreOption struct {
	MaxStoreCount int
	StoreSize     abi.ChainEpoch
}

type Splitstore struct {
	path          string
	maxStoreCount int
	storeSize     abi.ChainEpoch
	stores        []InnerBlockstore

	epochToAppend abi.ChainEpoch
	epochToClean  abi.ChainEpoch
}

var _ blockstore.Blockstore = (*Splitstore)(nil)

func NewSplitstore(path string, initStore blockstore.Blockstore, opts ...SplitstoreOption) (*Splitstore, error) {
	opt := SplitstoreOption{
		MaxStoreCount: 3,
		StoreSize:     2 * policy.ChainFinality,
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
	}

	// scan for stores
	bs, err := scan(path)
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		// init a placeholder and first store
		innerStore, err := newInnerStore(path, 0, "")
		if err != nil {
			return nil, err
		}
		bs = append(bs, innerStore)
	}

	if !bs[len(bs)-1].Base().Defined() {
		bs[len(bs)-1].Blockstore = initStore
	}

	for i := range bs {
		// take the last maxStoreCount  stores
		if i >= len(bs)-ss.maxStoreCount {
			ss.stores = append(ss.stores, bs[i])
		}
	}

	return ss, nil
}

// HeadChange subscribe to head change, schedule for new store and delete old
func (ss *Splitstore) HeadChange(_, apply []*types.TipSet) error {
	for _, ts := range apply {
		height := ts.Height()

		if height >= ss.epochToClean && len(ss.stores) > ss.maxStoreCount {
			storeToDrop := ss.stores[0]

			// block transfer
			tempStore := NewComposeStore(storeToDrop, ss.stores[0])
			v := NewSyncVisitor()
			err := WalkChain(context.Background(), tempStore, ss.stores[1].Base(), v, ss.storeSize)
			if err != nil {
				return fmt.Errorf("block transfer: %w", err)
			}

			ss.stores = ss.stores[1:]

			// close and  clean
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
		}

		if height >= ss.epochToAppend {
			ss.epochToAppend = height + ss.storeSize
			cleanDelay := ss.storeSize / 2
			ss.epochToClean = height + cleanDelay
			tskCid, err := ts.Key().Cid()
			if err != nil {
				return err
			}
			store, err := newInnerStore(ss.path, int64(height), tskCid.String())
			if err != nil {
				return err
			}
			ss.stores = append(ss.stores, store)
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

func (ss *Splitstore) Close() error {
	for i := range ss.stores {
		if closer, ok := ss.stores[i].(Closer); ok {
			err := closer.Close()
			if err != nil {
				return fmt.Errorf("close %dth store: %w", i, err)
			}
		}
	}
	return nil
}

func (ss *Splitstore) composeStore() blockstore.Blockstore {
	bs := make([]blockstore.Blockstore, len(ss.stores))
	for i := range ss.stores {
		bs[i] = ss.stores[i]
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
		var height int64
		var cidString string
		height, cidString, err = extractHeightAndCid(name)
		if err != nil {
			continue
		}

		innerStore, err := func() (*InnerBlockstoreImpl, error) {
			storePath := filepath.Join(path, name)
			cleanup := CleanImpl(func() error {
				return os.RemoveAll(storePath)
			})

			if cidString == "init" {
				// place holder for init store
				return &InnerBlockstoreImpl{
					Blockstore: nil,
					Cleaner:    cleanup,
					BaseKeeper: BaseKeeperImpl(func() cid.Cid {
						return cid.Undef
					}),
				}, err
			}

			baseCid := cid.MustParse(cidString)
			opt, err := blockstore.BadgerBlockstoreOptions(storePath, false)
			if err != nil {
				return nil, err
			}
			opt.Prefix = bstore.BlockPrefix.String()
			store, err := blockstore.Open(opt)
			if err != nil {
				return nil, err
			}
			return &InnerBlockstoreImpl{
				Blockstore: store,
				Cleaner:    cleanup,
				BaseKeeper: BaseKeeperImpl(func() cid.Cid {
					return baseCid
				}),
			}, nil
		}()
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

func newInnerStore(path string, height int64, c string) (*InnerBlockstoreImpl, error) {
	isEmpty := false
	if c == "" {
		c = "init"
		isEmpty = true
	}

	storePath := filepath.Join(path, fmt.Sprintf("base_%d_%s.db", height, c))
	var store blockstore.Blockstore
	baseCid := cid.Undef

	if isEmpty {
		// place holder for init store
		err := os.Mkdir(storePath, 0777)
		if err != nil {
			return nil, err
		}
	} else {
		opt, _ := blockstore.BadgerBlockstoreOptions(storePath, false)
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
			return baseCid
		}),
	}, nil
}

func extractHeightAndCid(s string) (int64, string, error) {
	re := regexp.MustCompile(`base_(\d+)_(\w+)\.db`)
	match := re.FindStringSubmatch(s)
	if len(match) != 3 {
		return 0, "", fmt.Errorf("failed to extract height and cid from %s", s)
	}
	height, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, "", err
	}
	return height, match[2], nil
}
