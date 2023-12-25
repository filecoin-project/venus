package splitstore

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/logging"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
)

var log = logging.New("splitstore")

// ComposeStore compose two block store into one
// Read: firstly from primary store, if not exist in primary will try secondary and rewrite to primary store
// Write: write to primary store only
// Delete: Delete from all store
type ComposeStore struct {
	shouldSync bool
	primary    blockstore.Blockstore
	secondary  blockstore.Blockstore
}

// NewComposeStore create a new ComposeStore with a list of blockstore
// low priority come first
func NewComposeStore(bs ...blockstore.Blockstore) blockstore.Blockstore {
	switch len(bs) {
	case 0:
		return nil
	case 1:
		return bs[0]
	}
	return Compose(bs...)
}

func Compose(bs ...blockstore.Blockstore) *ComposeStore {
	switch len(bs) {
	case 0:
		return nil
	case 1:
		return &ComposeStore{
			shouldSync: false,
			primary:    bs[0],
			secondary:  bs[0],
		}
	}

	ret := &ComposeStore{
		shouldSync: false,
		primary:    bs[1],
		secondary:  bs[0],
	}
	for i := 2; i < len(bs); i++ {
		ret = &ComposeStore{
			shouldSync: i == len(bs)-1,
			primary:    bs[i],
			secondary:  ret,
		}
	}

	return ret
}

// AllKeysChan implements blockstore.Blockstore.
func (cs *ComposeStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	ctx, cancel := context.WithCancel(ctx)

	primaryCh, err := cs.primary.AllKeysChan(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	secondaryCh, err := cs.secondary.AllKeysChan(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	seen := cid.NewSet()
	ch := make(chan cid.Cid, 8) // buffer is arbitrary, just enough to avoid context switches
	go func() {
		defer cancel()
		defer close(ch)

		for _, in := range []<-chan cid.Cid{primaryCh, secondaryCh} {
			for c := range in {
				// ensure we only emit each key once
				if !seen.Visit(c) {
					continue
				}

				select {
				case ch <- c:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

// DeleteBlock implements blockstore.Blockstore.
func (cs *ComposeStore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	if err := cs.secondary.DeleteBlock(ctx, c); err != nil {
		return err
	}
	return cs.primary.DeleteBlock(ctx, c)
}

// DeleteMany implements blockstore.Blockstore.
func (cs *ComposeStore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	// primary and secondly can be both incomplete
	// don't try to batch delete
	return fmt.Errorf("delete many not implemented on compose store; don't do this")
}

// Flush implements blockstore.Blockstore.
func (cs *ComposeStore) Flush(ctx context.Context) error {
	if err := cs.secondary.Flush(ctx); err != nil {
		return err
	}
	return cs.primary.Flush(ctx)
}

// Get implements blockstore.Blockstore.
func (cs *ComposeStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	has, err := cs.primary.Has(ctx, c)
	if err != nil {
		return nil, err
	}
	if has {
		return cs.primary.Get(ctx, c)
	}

	b, err := cs.secondary.Get(ctx, c)
	if err != nil {
		return nil, err
	}

	cs.sync(ctx, c, b)

	return b, nil
}

// GetSize implements blockstore.Blockstore.
func (cs *ComposeStore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	s, err := cs.primary.GetSize(ctx, c)
	if err == nil {
		return s, nil
	}
	if !ipld.IsNotFound(err) {
		return 0, err
	}

	cs.sync(ctx, c, nil)
	return cs.secondary.GetSize(ctx, c)
}

// Has implements blockstore.Blockstore.
func (cs *ComposeStore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	has, err := cs.primary.Has(ctx, c)
	if err != nil {
		return false, err
	}
	if !has {
		has, err = cs.secondary.Has(ctx, c)
		if err != nil {
			return false, err
		}
		if has {
			cs.sync(ctx, c, nil)
		}
	}

	return has, nil
}

// HashOnRead implements blockstore.Blockstore.
func (cs *ComposeStore) HashOnRead(enabled bool) {
	cs.primary.HashOnRead(enabled)
	cs.secondary.HashOnRead(enabled)
}

// Put implements blockstore.Blockstore.
func (cs *ComposeStore) Put(ctx context.Context, b blocks.Block) error {
	return cs.primary.Put(ctx, b)
}

// PutMany implements blockstore.Blockstore.
func (cs *ComposeStore) PutMany(ctx context.Context, bs []blocks.Block) error {
	return cs.primary.PutMany(ctx, bs)
}

// View implements blockstore.Blockstore.
func (cs *ComposeStore) View(ctx context.Context, c cid.Cid, cb func([]byte) error) error {
	err := cs.primary.View(ctx, c, cb)
	if err == nil {
		return nil
	}
	if !ipld.IsNotFound(err) {
		return err
	}
	cs.sync(ctx, c, nil)
	return cs.secondary.View(ctx, c, cb)

}

// sync sync block from secondly to primary
func (cs *ComposeStore) sync(ctx context.Context, c cid.Cid, b blocks.Block) {
	if !cs.shouldSync {
		return
	}

	go func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if b == nil {
			var err error
			b, err = cs.secondary.Get(ctx, c)
			if err != nil {
				// it is ok to ignore the err
				return
			}
		}

		err := cs.primary.Put(ctx, b)
		if err != nil {
			log.Warnf("put block(%s) to primary store: %w", b.Cid(), err)
		}
	}()
}

var _ blockstore.Blockstore = (*ComposeStore)(nil)
