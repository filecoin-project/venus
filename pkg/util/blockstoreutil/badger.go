package blockstoreutil

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/keytransform"
	"github.com/ipfs/go-ipfs-blockstore"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	"go.uber.org/zap"
)

var (
	// ErrBlockstoreClosed is returned from blockstore operations after
	// the blockstore has been closed.
	ErrBlockstoreClosed = fmt.Errorf("badger blockstore closed")
)

// aliases to mask badger dependencies.
const (
	// FileIO is equivalent to badger/options.FileIO.
	FileIO = options.FileIO
	// MemoryMap is equivalent to badger/options.MemoryMap.
	MemoryMap = options.MemoryMap
	// LoadToRAM is equivalent to badger/options.LoadToRAM.
	LoadToRAM = options.LoadToRAM
)

// Options embeds the badger options themselves, and augments them with
// blockstore-specific options.
type Options struct {
	badger.Options

	// Prefix is an optional prefix to prepend to keys. Default: "".
	Prefix string
}

func DefaultOptions(path string) Options {
	return Options{
		Options: badger.DefaultOptions(path),
		Prefix:  "",
	}
}

// BadgerBlockstoreOptions returns the badger options to apply for the provided
// domain.
func BadgerBlockstoreOptions(path string, readonly bool) (Options, error) {

	opts := DefaultOptions(path)

	// Due to legacy usage of blockstore.blockstore, over a datastore, all
	// blocks are prefixed with this namespace. In the future, this can go away,
	// in order to shorten keys, but it'll require a migration.
	opts.Prefix = ""

	// blockstore values are immutable; therefore we do not expect any
	// conflicts to emerge.
	opts.DetectConflicts = false

	// This is to optimize the database on close so it can be opened
	// read-only and efficiently queried. We don't do that and hanging on
	// stop isn't nice.
	opts.CompactL0OnClose = false

	// The alternative is "crash on start and tell the user to fix it". This
	// will truncate corrupt and unsynced data, which we don't guarantee to
	// persist anyways.
	opts.Truncate = true

	// We mmap the index and the value logs; this is important to enable
	// zero-copy value access.
	opts.ValueLogLoadingMode = FileIO
	opts.TableLoadingMode = FileIO

	// Embed only values < 128 bytes in the LSM tree; larger values are stored
	// in value logs.
	opts.ValueThreshold = 128

	// Default table size is already 64MiB. This is here to make it explicit.
	opts.MaxTableSize = 64 << 20

	// NOTE: The chain blockstore doesn't require any GC (blocks are never
	// deleted). This will change if we move to a tiered blockstore.

	opts.ReadOnly = readonly

	return opts, nil
}

// badgerLogger is a local wrapper for go-log to make the interface
// compatible with badger.Logger (namely, aliasing Warnf to Warningf)
type badgerLogger struct {
	*zap.SugaredLogger // skips 1 caller to get useful line info, skipping over badger.Options.

	skip2 *zap.SugaredLogger // skips 2 callers, just like above + this logger.
}

// Warningf is required by the badger logger APIs.
func (b *badgerLogger) Warningf(format string, args ...interface{}) {
	b.skip2.Warnf(format, args...)
}

const (
	stateOpen int64 = iota
	stateClosing
	stateClosed
)

// blockstore is a badger-backed IPLD blockstore.
//
// NOTE: once Close() is called, methods will try their best to return
// ErrBlockstoreClosed. This will guaranteed to happen for all subsequent
// operation calls after Close() has returned, but it may not happen for
// operations in progress. Those are likely to fail with a different error.
type BadgerBlockstore struct {
	DB *badger.DB

	// state is guarded by atomic.
	state int64

	keyTransform *keytransform.PrefixTransform

	cache IBlockCache
}

var _ blockstore.Blockstore = (*BadgerBlockstore)(nil)
var _ blockstore.Viewer = (*BadgerBlockstore)(nil)
var _ io.Closer = (*BadgerBlockstore)(nil)

// Open creates a new badger-backed blockstore, with the supplied options.
func Open(opts Options) (*BadgerBlockstore, error) {
	opts.Logger = &badgerLogger{
		SugaredLogger: log.Desugar().WithOptions(zap.AddCallerSkip(1)).Sugar(),
		skip2:         log.Desugar().WithOptions(zap.AddCallerSkip(2)).Sugar(),
	}
	keyTransform := &keytransform.PrefixTransform{
		Prefix: ds.NewKey(opts.Prefix),
	}
	db, err := badger.Open(opts.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger blockstore: %w", err)
	}
	cache := NewLruCache(1000 * 10000)
	bs := &BadgerBlockstore{
		DB:           db,
		keyTransform: keyTransform,
		cache:        cache,
	}
	return bs, nil
}

// Close closes the store. If the store has already been closed, this noops and
// returns an error, even if the first closure resulted in error.
func (b *BadgerBlockstore) Close() error {
	if !atomic.CompareAndSwapInt64(&b.state, stateOpen, stateClosing) {
		return nil
	}

	defer atomic.StoreInt64(&b.state, stateClosed)
	return b.DB.Close()
}

func (b *BadgerBlockstore) ReadonlyDatastore() *TxBlockstore {
	return &TxBlockstore{
		cache:        b.cache,
		tx:           b.DB.NewTransaction(false),
		keyTransform: b.keyTransform,
	}
}

// View implements blockstore.Viewer, which leverages zero-copy read-only
// access to values.
func (b *BadgerBlockstore) View(cid cid.Cid, fn func([]byte) error) error {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return ErrBlockstoreClosed
	}

	key := b.ConvertKey(cid)
	return b.DB.View(func(txn *badger.Txn) error {
		switch item, err := txn.Get(key.Bytes()); err {
		case nil:
			return item.Value(fn)
		case badger.ErrKeyNotFound:
			return blockstore.ErrNotFound
		default:
			return fmt.Errorf("failed to view block from badger blockstore: %w", err)
		}
	})
}

// Has implements blockstore.Has.
func (b *BadgerBlockstore) Has(cid cid.Cid) (bool, error) {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return false, ErrBlockstoreClosed
	}

	key := b.ConvertKey(cid)
	if b.cache != nil {
		if _, has := b.cache.Get(key.String()); has {
			return true, nil
		}
	}

	err := b.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key.Bytes())
		return err
	})

	switch err {
	case badger.ErrKeyNotFound:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, fmt.Errorf("failed to check if block exists in badger blockstore: %w", err)
	}
}

// Get implements blockstore.Get.
func (b *BadgerBlockstore) Get(cid cid.Cid) (blocks.Block, error) {
	if !cid.Defined() {
		return nil, blockstore.ErrNotFound
	}

	if atomic.LoadInt64(&b.state) != stateOpen {
		return nil, ErrBlockstoreClosed
	}

	key := b.ConvertKey(cid)
	if b.cache != nil {
		if val, has := b.cache.Get(key.String()); has {
			return val.(blocks.Block), nil
		}
	}

	//migrate
	//todo just for test
	var val []byte
	err := b.DB.View(func(txn *badger.Txn) error {
		switch item, err := txn.Get(key.Bytes()); err {
		case nil:
			val, err = item.ValueCopy(nil)
			return err
		case badger.ErrKeyNotFound:
			return blockstore.ErrNotFound
		default:
			return fmt.Errorf("failed to get block from badger blockstore: %w", err)
		}
	})
	if err != nil {
		return nil, err
	}
	blk, err := blocks.NewBlockWithCid(val, cid)
	if err != nil {
		return nil, err
	}

	b.cache.Add(key.String(), blk)
	return blk, nil
}

// GetSize implements blockstore.GetSize.
func (b *BadgerBlockstore) GetSize(cid cid.Cid) (int, error) {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return -1, ErrBlockstoreClosed
	}

	key := b.ConvertKey(cid)
	if b.cache != nil {
		if val, has := b.cache.Get(key.String()); has {
			return len(val.(blocks.Block).RawData()), nil
		}
	}

	var size int
	err := b.DB.View(func(txn *badger.Txn) error {
		switch item, err := txn.Get(key.Bytes()); err {
		case nil:
			size = int(item.ValueSize())
		case badger.ErrKeyNotFound:
			return blockstore.ErrNotFound
		default:
			return fmt.Errorf("failed to get block size from badger blockstore: %w", err)
		}
		return nil
	})
	if err != nil {
		size = -1
	}
	return size, err
}

// Put implements blockstore.Put.
func (b *BadgerBlockstore) Put(block blocks.Block) error {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return ErrBlockstoreClosed
	}

	key := b.ConvertKey(block.Cid())
	if _, ok := b.cache.Get(key.String()); ok {
		return nil
	}

	err := b.DB.Update(func(txn *badger.Txn) error {
		err := txn.Set(key.Bytes(), block.RawData())
		if err == nil {
			b.cache.Add(key.String(), block)
		}
		return err
	})
	if err != nil {
		err = fmt.Errorf("failed to put block in badger blockstore: %w", err)
	}
	return err
}

// PutMany implements blockstore.PutMany.
func (b *BadgerBlockstore) PutMany(blks []blocks.Block) error {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return ErrBlockstoreClosed
	}

	batch := b.DB.NewWriteBatch()
	defer batch.Cancel()

	flushToCache := map[string]blocks.Block{}
	for _, block := range blks {
		key := b.ConvertKey(block.Cid())
		if _, ok := b.cache.Get(key.String()); ok {
			continue
		}

		if err := batch.Set(key.Bytes(), block.RawData()); err != nil {
			return err
		}

		flushToCache[key.String()] = block
	}

	err := batch.Flush()
	if err != nil {
		err = fmt.Errorf("failed to put blocks in badger blockstore: %w", err)
	}
	//flush to cache
	for k, v := range flushToCache {
		b.cache.Add(k, v)
	}
	return err
}

// DeleteBlock implements blockstore.DeleteBlock.
func (b *BadgerBlockstore) DeleteBlock(cid cid.Cid) error {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return ErrBlockstoreClosed
	}

	key := b.ConvertKey(cid)
	return b.DB.Update(func(txn *badger.Txn) error {
		err := txn.Delete(key.Bytes())
		if err == nil {
			b.cache.Remove(key.String())
		}
		return err
	})
}

// AllKeysChan implements blockstore.AllKeysChan.
func (b *BadgerBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	if atomic.LoadInt64(&b.state) != stateOpen {
		return nil, ErrBlockstoreClosed
	}

	txn := b.DB.NewTransaction(false)
	opts := badger.IteratorOptions{PrefetchSize: 100}
	iter := txn.NewIterator(opts)

	ch := make(chan cid.Cid)
	go func() {
		defer close(ch)
		defer iter.Close()

		// NewCidV1 makes a copy of the multihash buffer, so we can reuse it to
		// contain allocs.
		for iter.Rewind(); iter.Valid(); iter.Next() {
			if ctx.Err() != nil {
				return // context has fired.
			}
			if atomic.LoadInt64(&b.state) != stateOpen {
				// open iterators will run even after the database is closed...
				return // closing, yield.
			}
			k := iter.Item().Key()
			// need to convert to key.Key using key.KeyFromDsKey.
			bk, err := dshelp.BinaryFromDsKey(ds.RawKey(string(k)))
			if err != nil {
				log.Warnf("error parsing key from binary: %s", err)
				continue
			}
			cidKey := cid.NewCidV1(cid.Raw, bk)
			select {
			case <-ctx.Done():
				return
			case ch <- cidKey:
			}
		}
	}()

	return ch, nil
}

// HashOnRead implements blockstore.HashOnRead. It is not supported by this
// blockstore.
func (b *BadgerBlockstore) HashOnRead(_ bool) {
	log.Warnf("called HashOnRead on badger blockstore; function not supported; ignoring")
}

func (b *BadgerBlockstore) ConvertKey(cid cid.Cid) datastore.Key {
	key := dshelp.MultihashToDsKey(cid.Hash())
	return b.keyTransform.ConvertKey(key)
}
