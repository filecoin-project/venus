package storage

import (
	"context"
	"errors"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipfs/go-ipld-format"
)

// TODO: limit memory footprint
// TODO: implement ipld.Store

// VMStorage implements a content-addressable store for the VM.
type VMStorage struct {
	blockstore       blockstore.Blockstore
	writeBuffer      map[cid.Cid]ipld.Node
	readCache        map[cid.Cid]blocks.Block
	readCacheEnabled bool
}

var _ = cbor.IpldStore((*VMStorage)(nil))

// ErrNotFound is returned by storage when no object matches a requested Cid.
var ErrNotFound = errors.New("object not found")

// SerializationError is returned by storage when de/serialization of the object fails.
type SerializationError struct {
	error
}

// NewStorage creates a new VMStorage.
func NewStorage(bs blockstore.Blockstore) *VMStorage {
	return &VMStorage{
		blockstore:       bs,
		writeBuffer:      map[cid.Cid]ipld.Node{},
		readCache:        map[cid.Cid]blocks.Block{},
		readCacheEnabled: false,
	}
}

// SetReadCache enable/disables the read chache.
func (s *VMStorage) SetReadCache(enabled bool) {
	s.readCacheEnabled = enabled
}

//**********ipld interface impl**************//
func (s *VMStorage) Get(ctx context.Context, c cid.Cid, obj interface{}) error {
	//fmt.Println("storage get ", c.String(), " ", unsafe.Pointer(s))
	raw, err := s.GetRaw(ctx, c)
	if err != nil {
		return err
	}

	err = encoding.Decode(raw, obj)
	if err != nil {
		return SerializationError{err}
	}
	return nil
}

func (s *VMStorage) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	nd, err := s.toNode(v)
	if err != nil {
		return cid.Undef, SerializationError{err}
	}
	// append the object to the buffer
	cid := nd.Cid()
	s.writeBuffer[cid] = nd
	//fmt.Println("storage put ", cid.String())
	return cid, nil
}

//**********ipld interface impl end**************//

// Put stores object and returns it's content-addressable ID.
func (s *VMStorage) PutWithLen(ctx context.Context, obj interface{}) (cid.Cid, int, error) {
	nd, err := s.toNode(obj)
	if err != nil {
		return cid.Undef, 0, SerializationError{err}
	}

	// append the object to the buffer
	cid := nd.Cid()
	s.writeBuffer[cid] = nd
	//fmt.Println("storage put with len ", cid.String())
	return cid, len(nd.RawData()), nil
}

// CidOf returns the Cid of the object without storing it.
func (s *VMStorage) CidOf(obj interface{}) (cid.Cid, error) {
	nd, err := s.toNode(obj)
	if err != nil {
		return cid.Undef, err
	}
	return nd.Cid(), nil
}

// Get loads the object based on its content-addressable ID.
func (s *VMStorage) GetWithLen(ctx context.Context, cid cid.Cid, obj interface{}) (int, error) {
	raw, err := s.GetRaw(ctx, cid)
	if err != nil {
		return 0, err
	}
	//fmt.Println("storage get with len ", cid.String())
	err = encoding.Decode(raw, obj)
	if err != nil {
		return 0, SerializationError{err}
	}
	return len(raw), nil
}

// GetRaw retrieves the raw bytes stored, returns true if it exists.
func (s *VMStorage) GetRaw(ctx context.Context, cid cid.Cid) ([]byte, error) {
	// attempt to read from write buffer first
	n, ok := s.writeBuffer[cid]
	if ok {
		// decode the object
		return n.RawData(), nil
	}

	if s.readCacheEnabled {
		// attempt to read from the read cache
		n, ok := s.readCache[cid]
		if ok {
			// decode the object
			return n.RawData(), nil
		}
	}

	// read from store
	blk, err := s.blockstore.Get(cid)
	if err != nil {
		if err == blockstore.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if s.readCacheEnabled {
		// add object to read cache
		s.readCache[cid] = blk
	}

	return blk.RawData(), nil
}

// Flush writes all the in-memory held objects down to the store.
//
// This will automatically clear the write buffer when returning without error.
//
// If the read cache is enabled, the flushed objects will be read from cache.
func (s *VMStorage) Flush() error {
	// extract list of blocks for the underlying store from our internal map
	blks := make([]blocks.Block, 0, len(s.writeBuffer))
	for _, nd := range s.writeBuffer {
		//fmt.Println("storage flush buffer ", nd.Cid())
		blks = append(blks, nd)
	}

	// From https://github.com/dgraph-io/badger/issues/441: "a txn should not exceed the size of a single memtable"
	// Default badger.DefaultOptions.MaxTableSize is 64Mib
	// Pushing this hard would require measuring the size of each block and also accounting for badger object overheads.
	// 1024 would give us very generous room for 64Kib per object.
	maxBatchSize := 1024

	// Write at most maxBatchSize objects to store at a time
	remaining := blks
	for len(remaining) > 0 {
		last := min(len(remaining), maxBatchSize)
		if err := s.blockstore.PutMany(remaining[:last]); err != nil {
			return err
		}
		remaining = remaining[last:]
	}

	if s.readCacheEnabled {
		// move objects to read cache
		for cid, nd := range s.writeBuffer {
			s.readCache[cid] = nd
		}
	}
	// clear write buffer
	s.ClearWriteBuffer()
	return nil
}

// ClearWriteBuffer drops all the pending writes.
func (s *VMStorage) ClearWriteBuffer() {
	s.writeBuffer = map[cid.Cid]ipld.Node{}
}

// Clear will clear all buffers and caches.
//
// WARNING: thil will NOT flush the pending writes to the store.
func (s *VMStorage) Clear() {
	s.writeBuffer = map[cid.Cid]ipld.Node{}
	s.readCache = map[cid.Cid]blocks.Block{}
}

// Put adds a node to temporary storage by id.
func (s *VMStorage) toNode(v interface{}) (ipld.Node, error) {
	var nd format.Node
	var err error
	if blk, ok := v.(blocks.Block); ok {
		// optimize putting blocks
		nd, err = cbor.DecodeBlock(blk)
	} else {
		var raw []byte
		raw, err = encoding.Encode(v)
		if err != nil {
			return nil, err
		}

		nd, err = cbor.Decode(raw, constants.DefaultHashFunction, -1)
	}
	if err != nil {
		return nil, err
	}
	return nd, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
