package blockstore

import (
	"context"
	llog "log"
	"os"

	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
)

type LogStore struct {
	logger *llog.Logger
	bs     Blockstore
}

// DeleteMany implements Blockstore.
func (l *LogStore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	for _, c := range cids {
		l.log(c, "delete", "")
	}
	return l.bs.DeleteMany(ctx, cids)
}

// Flush implements Blockstore.
func (l *LogStore) Flush(ctx context.Context) error {
	l.logger.Println("flush")
	return l.bs.Flush(ctx)
}

// View implements Blockstore.
func (l *LogStore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	l.log(cid, "view", "")
	return l.bs.View(ctx, cid, callback)
}

// AllKeysChan implements blockstore.Blockstore.
func (l *LogStore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return l.AllKeysChan(ctx)
}

// DeleteBlock implements blockstore.Blockstore.
func (l *LogStore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	l.log(c, "delete", "")
	return l.bs.DeleteBlock(ctx, c)
}

// Get implements blockstore.Blockstore.
func (l *LogStore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	l.log(c, "get", "")
	return l.bs.Get(ctx, c)
}

// GetSize implements blockstore.Blockstore.
func (l *LogStore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	l.log(c, "getsize", "")
	return l.bs.GetSize(ctx, c)
}

// Has implements blockstore.Blockstore.
func (l *LogStore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	l.log(c, "has", "")
	return l.bs.Has(ctx, c)
}

// HashOnRead implements blockstore.Blockstore.
func (l *LogStore) HashOnRead(enabled bool) {
	l.bs.HashOnRead(enabled)
}

// Put implements blockstore.Blockstore.
func (l *LogStore) Put(ctx context.Context, b blocks.Block) error {
	l.log(b.Cid(), "put", "")
	return l.bs.Put(ctx, b)
}

// DeleteMany implements blockstore.Blockstore.
func (l *LogStore) PutMany(ctx context.Context, bs []blocks.Block) error {
	for _, b := range bs {
		l.log(b.Cid(), "put", "")
	}
	return l.bs.PutMany(ctx, bs)
}

var _ Blockstore = (*LogStore)(nil)

func NewLogStore(path string, bs Blockstore) *LogStore {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	logger := llog.New(file, "", llog.LstdFlags)
	logger.Println("log store opened")
	return &LogStore{
		logger: logger,
		bs:     bs,
	}
}

func (l *LogStore) log(c cid.Cid, op, msg string) {
	l.logger.Printf("%s %s %s", c.String(), op, msg)
}
