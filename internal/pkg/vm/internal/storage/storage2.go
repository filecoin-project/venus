package storage

import (
	"bytes"
	"errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipfs/go-ipld-format"
	cbg "github.com/whyrusleeping/cbor-gen"
)

// VMStorage implements a content-addressable store for the VM.
type VMStorage struct {
	blockstore       blockstore.Blockstore
	writeBuffer      map[cid.Cid]ipld.Node
	readCache        map[cid.Cid]blocks.Block
	readCacheEnabled bool
}

// ErrNotFound is returned by storage when no object matches a requested Cid
var ErrNotFound = errors.New("object not found")

// NewStorage creates a new VMStorage.
func NewStorage(bs blockstore.Blockstore) VMStorage {
	return VMStorage{
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

// Put stores object and returns it's content-addressable ID.
func (s *VMStorage) Put(obj interface{}) (cid.Cid, error) {
	nd, err := s.toNode(obj)
	if err != nil {
		return cid.Undef, err
	}

	// append the object to the buffer
	cid := nd.Cid()
	s.writeBuffer[cid] = nd

	return cid, nil
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
func (s *VMStorage) Get(cid cid.Cid, obj interface{}) error {
	raw, err := s.GetRaw(cid)
	if err != nil {
		return err
	}
	return encoding.Decode(raw, obj)
}

// GetRaw retrieves the raw bytes stored, returns true if it exists.
func (s *VMStorage) GetRaw(cid cid.Cid) ([]byte, error) {
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
		blks = append(blks, nd)
	}

	// write objects to store
	if err := s.blockstore.PutMany(blks); err != nil {
		return err
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
	// Dragons: i lifted this to make it work for now, need to review if all of this code is needed
	var nd format.Node
	var err error
	if blk, ok := v.(blocks.Block); ok {
		// optimize putting blocks
		nd, err = cbor.DecodeBlock(blk)
	} else if raw, ok := v.([]byte); ok {
		nd, err = cbor.Decode(raw, types.DefaultHashFunction, -1)

	} else if cm, ok := v.(cbg.CBORMarshaler); ok {
		// TODO: Remote this clause once
		// https://github.com/ipfs/go-ipld-cbor/pull/64
		// is merged and cbor.WrapObject can be called directly on objects that
		// support fastpath marshalling / unmarshalling
		buf := new(bytes.Buffer)
		err = cm.MarshalCBOR(buf)
		if err == nil {
			nd, err = cbor.Decode(buf.Bytes(), types.DefaultHashFunction, -1)
		}
	} else {
		nd, err = cbor.WrapObject(v, types.DefaultHashFunction, -1)
	}
	if err != nil {
		return nil, internal.Errors[internal.ErrDecode]
	}
	return nd, nil
}
