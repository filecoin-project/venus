package storagemap

// Content-addressed storage API.
// The storage API has a few goals:
// 1. Provide access to content-addressed persistent storage
// 2. Stage this storage to permit rollback on message failure.
// 3. Isolate staged changes across actors to reduce concurrency/message ordering issues.
// 4. Associate storage with actors by managing actor.Head.

import (
	"bytes"
	"errors"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipfs/go-ipld-format"
	cbg "github.com/whyrusleeping/cbor-gen"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	vmerrors "github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

// StorageMap manages Storages.
type StorageMap interface {
	NewStorage(addr address.Address, actor *actor.Actor) runtime.Storage
	Flush() error
}

// storageMap implements StorageMap as a map of `Storage` structs keyed by actor address.
type storageMap struct {
	blockstore blockstore.Blockstore
	storageMap map[address.Address]storage
}

// storage is a place to hold chunks that are created while processing a block.
type storage struct {
	actor      *actor.Actor
	chunks     map[cid.Cid]ipld.Node
	blockstore blockstore.Blockstore
}

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return &storageMap{
		blockstore: bs,
		storageMap: map[address.Address]storage{},
	}
}

var _ StorageMap = &storageMap{}

// NewStorage gets or creates a `runtime.Storage` for the given address
// storage updates the given actor's storage by updating its Head property.
// The instance of actor passed into this method needs to be the instance ultimately
// persisted.
func (s *storageMap) NewStorage(addr address.Address, actor *actor.Actor) runtime.Storage {
	st, ok := s.storageMap[addr]
	if ok {
		// Return a hybrid storage with the pre-existing chunks, but the given instance of the actor.
		// This ensures changes made to the actor appear in the state tree cache.
		st = storage{
			actor:      actor,
			chunks:     st.chunks,
			blockstore: s.blockstore,
		}
	} else {
		st = storage{
			chunks:     map[cid.Cid]ipld.Node{},
			actor:      actor,
			blockstore: s.blockstore,
		}
	}

	s.storageMap[addr] = st

	return st
}

// Flush saves all valid staged changes to the datastore
func (s *storageMap) Flush() error {
	for _, st := range s.storageMap {
		err := st.Flush()
		if err != nil {
			return err
		}
	}

	return nil
}

var _ runtime.Storage = (*storage)(nil)

// ErrNotFound is returned by storage when no chunk in storage matches a requested Cid
var ErrNotFound = errors.New("chunk not found")

// Put adds a node to temporary storage by id.
func (s storage) Put(v interface{}) (cid.Cid, error) {
	nd, err := s.toNode(v)
	if err != nil {
		return cid.Undef, err
	}

	c := nd.Cid()
	s.chunks[c] = nd

	return c, nil
}

// CidOf returns the Cid of the object without storing it.
func (s storage) CidOf(v interface{}) (cid.Cid, error) {
	nd, err := s.toNode(v)
	if err != nil {
		return cid.Undef, err
	}
	return nd.Cid(), nil
}

// Get retrieves a chunk from either temporary storage or its backing store.
// If the chunk is not found in storage, a vm.ErrNotFound error is returned.
func (s storage) Get(cid cid.Cid) ([]byte, error) {
	n, ok := s.chunks[cid]
	if ok {
		return n.RawData(), nil
	}

	blk, err := s.blockstore.Get(cid)
	if err != nil {
		if err == blockstore.ErrNotFound {
			return []byte{}, ErrNotFound
		}
		return []byte{}, err
	}

	return blk.RawData(), nil
}

// Commit updates the head of the current actor to the given cid.
// The new cid must be the content id of a chunk put in storage.
// The given oldCid must match the cid of the current actor.
func (s storage) Commit(newCid cid.Cid, oldCid cid.Cid) error {
	// commit to initialize actor only permitted if Head and expected id are nil
	if oldCid.Defined() && s.actor.Head.Defined() && !oldCid.Equals(s.actor.Head) {
		return internal.Errors[internal.ErrStaleHead]
	} else if oldCid != s.actor.Head { // covers case where only one cid is nil
		return internal.Errors[internal.ErrStaleHead]
	}

	// validate completeness by traversing graph to find ids
	if _, err := s.liveDescendantIds(newCid); err != nil {
		return internal.Errors[internal.ErrDanglingPointer]
	}

	s.actor.Head = newCid

	return nil
}

// Head return the current head of the actor's memory
func (s storage) Head() cid.Cid {
	return s.actor.Head
}

// Flush write storage to underlying datastore
func (s *storage) Flush() error {
	liveIds, err := s.liveDescendantIds(s.actor.Head)
	if err != nil {
		return err
	}

	blks := make([]blocks.Block, 0, liveIds.Len())
	liveIds.ForEach(func(c cid.Cid) error { // nolint: errcheck
		blks = append(blks, s.chunks[c])
		return nil
	})

	return s.blockstore.PutMany(blks)
}

// Put adds a node to temporary storage by id.
func (s storage) toNode(v interface{}) (ipld.Node, error) {
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

// liveDescendantIds returns the ids of all chunks reachable from the given id for this storage.
// That is the given id , any links in the chunk referenced by the given id, or any links
// referenced from those links.
func (s storage) liveDescendantIds(id cid.Cid) (*cid.Set, error) {
	if !id.Defined() {
		return cid.NewSet(), nil
	}
	chunk, ok := s.chunks[id]
	if !ok {
		has, err := s.blockstore.Has(id)
		if err != nil {
			return nil, vmerrors.FaultErrorWrapf(err, "linked node, %s, missing from stage during flush", id)
		}

		// unstaged chunk that exists in datastore is valid, but halts recursion.
		if has {
			return cid.NewSet(), nil
		}

		return nil, vmerrors.NewFaultErrorf("linked node, %s, missing from storage during flush", id)
	}

	ids := cid.NewSet()
	ids.Add(id)

	for _, link := range chunk.Links() {
		linked, err := s.liveDescendantIds(link.Cid)
		if err != nil {
			return nil, err
		}
		linked.ForEach(func(id cid.Cid) error { // nolint: errcheck
			ids.Add(id)
			return nil
		})
	}

	return ids, nil
}
