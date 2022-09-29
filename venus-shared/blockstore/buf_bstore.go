package blockstoreutil

import (
	"context"
	"os"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("bufbs")

type BufferedBS struct {
	read  Blockstore
	write Blockstore

	readviewer  Viewer
	writeviewer Viewer
}

func NewBufferedBstore(base Blockstore) *BufferedBS {
	var buf Blockstore
	if os.Getenv("LOTUS_DISABLE_VM_BUF") == "iknowitsabadidea" {
		log.Warn("VM BLOCKSTORE BUFFERING IS DISABLED")
		buf = base
	} else {
		buf = NewTemporary()
	}

	bs := &BufferedBS{
		read:  base,
		write: buf,
	}
	if v, ok := base.(Viewer); ok {
		bs.readviewer = v
	}
	if v, ok := buf.(Viewer); ok {
		bs.writeviewer = v
	}
	if (bs.writeviewer == nil) != (bs.readviewer == nil) {
		log.Warnf("one of the stores is not viewable; running less efficiently")
	}
	return bs
}

func NewTieredBstore(r Blockstore, w Blockstore) *BufferedBS {
	return &BufferedBS{
		read:  r,
		write: w,
	}
}

var _ Blockstore = (*BufferedBS)(nil)
var _ Viewer = (*BufferedBS)(nil)

func (bs *BufferedBS) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	a, err := bs.read.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	b, err := bs.write.AllKeysChan(ctx)
	if err != nil {
		return nil, err
	}

	out := make(chan cid.Cid)
	go func() {
		defer close(out)
		for a != nil || b != nil {
			select {
			case val, ok := <-a:
				if !ok {
					a = nil
				} else {
					select {
					case out <- val:
					case <-ctx.Done():
						return
					}
				}
			case val, ok := <-b:
				if !ok {
					b = nil
				} else {
					select {
					case out <- val:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return out, nil
}

func (bs *BufferedBS) DeleteBlock(ctx context.Context, c cid.Cid) error {
	if err := bs.read.DeleteBlock(ctx, c); err != nil {
		return err
	}

	return bs.write.DeleteBlock(ctx, c)
}

func (bs *BufferedBS) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	if err := bs.read.DeleteMany(ctx, cids); err != nil {
		return err
	}

	return bs.write.DeleteMany(ctx, cids)
}

func (bs *BufferedBS) View(ctx context.Context, c cid.Cid, callback func([]byte) error) error {
	if bs.writeviewer == nil || bs.readviewer == nil {
		// one of the stores isn't Viewer; fall back to pure Get behaviour.
		blk, err := bs.Get(ctx, c)
		if err != nil {
			return err
		}
		return callback(blk.RawData())
	}

	// both stores are viewable.
	if err := bs.writeviewer.View(ctx, c, callback); ipld.IsNotFound(err) {
		// not found in write blockstore; fall through.
	} else {
		return err // propagate errors, or nil, i.e. found.
	}
	return bs.readviewer.View(ctx, c, callback)
}

func (bs *BufferedBS) Get(ctx context.Context, c cid.Cid) (block.Block, error) {
	if out, err := bs.write.Get(ctx, c); err != nil {
		if !ipld.IsNotFound(err) {
			return nil, err
		}
	} else {
		return out, nil
	}

	return bs.read.Get(ctx, c)
}

func (bs *BufferedBS) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	s, err := bs.read.GetSize(ctx, c)
	if ipld.IsNotFound(err) || s == 0 {
		return bs.write.GetSize(ctx, c)
	}

	return s, err
}

func (bs *BufferedBS) Put(ctx context.Context, blk block.Block) error {
	has, err := bs.read.Has(ctx, blk.Cid()) // TODO: consider dropping this check
	if err != nil {
		return err
	}

	if has {
		return nil
	}

	return bs.write.Put(ctx, blk)
}

func (bs *BufferedBS) Has(ctx context.Context, c cid.Cid) (bool, error) {
	has, err := bs.write.Has(ctx, c)
	if err != nil {
		return false, err
	}
	if has {
		return true, nil
	}

	return bs.read.Has(ctx, c)
}

func (bs *BufferedBS) HashOnRead(hor bool) {
	bs.read.HashOnRead(hor)
	bs.write.HashOnRead(hor)
}

func (bs *BufferedBS) PutMany(ctx context.Context, blks []block.Block) error {
	return bs.write.PutMany(ctx, blks)
}

func (bs *BufferedBS) Read() Blockstore {
	return bs.read
}

func (bs *BufferedBS) Write() Blockstore {
	return bs.write
}
