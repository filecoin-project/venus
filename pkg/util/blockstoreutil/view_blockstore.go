package blockstoreutil

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/keytransform"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
	ipld "github.com/ipfs/go-ipld-format"
)

var _ Blockstore = (*TxBlockstore)(nil)

type TxBlockstore struct {
	tx           *badger.Txn
	cache        IBlockCache
	keyTransform *keytransform.PrefixTransform
}

func (txBlockstore *TxBlockstore) DeleteBlock(ctx context.Context, cid cid.Cid) error {
	return errors.New("readonly blocksgtore")
}

func (txBlockstore *TxBlockstore) DeleteMany(ctx context.Context, cids []cid.Cid) error {
	return errors.New("readonly blocksgtore")
}

func (txBlockstore *TxBlockstore) Has(ctx context.Context, cid cid.Cid) (bool, error) {
	key := txBlockstore.ConvertKey(cid)
	if txBlockstore.cache != nil {
		if _, has := txBlockstore.cache.Get(key.String()); has {
			return true, nil
		}
	}

	_, err := txBlockstore.tx.Get(key.Bytes())
	switch err {
	case badger.ErrKeyNotFound:
		return false, nil
	case nil:
		return true, nil
	default:
		return false, fmt.Errorf("failed to check if block exists in badger blockstore: %w", err)
	}
}

func (txBlockstore *TxBlockstore) Get(ctx context.Context, cid cid.Cid) (blocks.Block, error) {
	if !cid.Defined() {
		return nil, ipld.ErrNotFound{Cid: cid}
	}

	key := txBlockstore.ConvertKey(cid)
	if txBlockstore.cache != nil {
		if val, has := txBlockstore.cache.Get(key.String()); has {
			return val.(blocks.Block), nil
		}
	}

	var val []byte
	var err error
	var item *badger.Item
	switch item, err = txBlockstore.tx.Get(key.Bytes()); err {
	case nil:
		val, err = item.ValueCopy(nil)
	case badger.ErrKeyNotFound:
		return nil, ipld.ErrNotFound{Cid: cid}
	default:
		return nil, fmt.Errorf("failed to get block from badger blockstore: %w", err)
	}
	if err != nil {
		return nil, err
	}

	blk, err := blocks.NewBlockWithCid(val, cid)
	if err != nil {
		return nil, err
	}

	txBlockstore.cache.Add(key.String(), blk)
	return blk, nil
}

func (txBlockstore *TxBlockstore) View(ctx context.Context, cid cid.Cid, callback func([]byte) error) error {
	if !cid.Defined() {
		return ipld.ErrNotFound{Cid: cid}
	}

	key := txBlockstore.ConvertKey(cid)
	if txBlockstore.cache != nil {
		if val, has := txBlockstore.cache.Get(key.String()); has {
			return callback(val.(blocks.Block).RawData())
		}
	}

	var val []byte
	var err error
	var item *badger.Item
	switch item, err = txBlockstore.tx.Get(key.Bytes()); err {
	case nil:
		val, err = item.ValueCopy(nil)
	case badger.ErrKeyNotFound:
		return ipld.ErrNotFound{Cid: cid}
	default:
		return fmt.Errorf("failed to get block from badger blockstore: %w", err)
	}
	if err != nil {
		return err
	}

	blk, err := blocks.NewBlockWithCid(val, cid)
	if err != nil {
		return err
	}

	txBlockstore.cache.Add(key.String(), blk)
	return callback(blk.RawData())
}

func (txBlockstore *TxBlockstore) GetSize(ctx context.Context, cid cid.Cid) (int, error) {
	key := txBlockstore.ConvertKey(cid)
	if txBlockstore.cache != nil {
		if val, has := txBlockstore.cache.Get(key.String()); has {
			return len(val.(blocks.Block).RawData()), nil
		}
	}

	var size int
	var err error
	var item *badger.Item
	switch item, err = txBlockstore.tx.Get(key.Bytes()); err {
	case nil:
		size = int(item.ValueSize())
	case badger.ErrKeyNotFound:
		return -1, ipld.ErrNotFound{Cid: cid}
	default:
		return -1, fmt.Errorf("failed to get block size from badger blockstore: %w", err)
	}
	return size, err
}

func (txBlockstore *TxBlockstore) Put(ctx context.Context, block blocks.Block) error {
	return errors.New("readonly blocksgtore")
}

func (txBlockstore *TxBlockstore) PutMany(ctx context.Context, blocks []blocks.Block) error {
	return errors.New("readonly blocksgtore")
}

func (txBlockstore *TxBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	opts := badger.IteratorOptions{PrefetchSize: 100}
	iter := txBlockstore.tx.NewIterator(opts)

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

			k := iter.Item().Key()
			// need to convert to key.Key using key.KeyFromDsKey.

			dsKey := txBlockstore.keyTransform.InvertKey(datastore.RawKey(string(k)))
			bk, err := dshelp.BinaryFromDsKey(dsKey)
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

func (txBlockstore *TxBlockstore) ConvertKey(cid cid.Cid) datastore.Key {
	key := dshelp.MultihashToDsKey(cid.Hash())
	return txBlockstore.keyTransform.ConvertKey(key)
}

func (txBlockstore *TxBlockstore) HashOnRead(enabled bool) {
	log.Warnf("called HashOnRead on badger blockstore; function not supported; ignoring")
}
