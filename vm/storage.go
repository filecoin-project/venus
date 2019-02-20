package vm

import (
	"errors"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	format "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	blocks "gx/ipfs/QmWoXtvgC8inqFkAATB7cp2Dax7XBi9VDvSg9RCCZufmRk/go-block-format"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	vmerrors "github.com/filecoin-project/go-filecoin/vm/errors"
)

// ErrNotFound is returned by storage when no chunk in storage matches a requested Cid
var ErrNotFound = errors.New("chunk not found")

// Content-addressed storage API.
// The storage API has a few goals:
// 1. Provide access to content-addressed persistent storage
// 2. Stage this storage to permit rollback on message failure.
// 3. Isolate staged changes across actors to reduce concurrency/message ordering issues.
// 4. Associate storage with actors by managing actor.Head.

// storageMap implements StorageMap as a map of Storage structs keyed by actor address.
type storageMap struct {
	blockstore blockstore.Blockstore
	storageMap map[address.Address]Storage
}

// StorageMap manages Storages.
type StorageMap interface {
	NewStorage(addr address.Address, actor *actor.Actor) Storage
	Flush() error
}

var _ StorageMap = &storageMap{}

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(bs blockstore.Blockstore) StorageMap {
	return &storageMap{
		blockstore: bs,
		storageMap: map[address.Address]Storage{},
	}
}

// NewStorage gets or creates a Storage for the given address
// Storage updates the given actor's storage by updating its Head property.
// The instance of actor passed into this method needs to be the instance ultimately
// persisted.
func (s *storageMap) NewStorage(addr address.Address, actor *actor.Actor) Storage {
	storage, ok := s.storageMap[addr]
	if ok {
		// Return a hybrid storage with the pre-existing chunks, but the given instance of the actor.
		// This ensures changes made to the actor appear in the state tree cache.
		storage = Storage{
			actor:      actor,
			chunks:     storage.chunks,
			blockstore: s.blockstore,
		}
	} else {
		storage = NewStorage(s.blockstore, actor)
	}

	s.storageMap[addr] = storage

	return storage
}

// Flush saves all valid staged changes to the datastore
func (s *storageMap) Flush() error {
	for _, storage := range s.storageMap {
		err := storage.Flush()
		if err != nil {
			return err
		}
	}

	return nil
}

// Storage is a place to hold chunks that are created while processing a block.
type Storage struct {
	actor      *actor.Actor
	chunks     map[cid.Cid]ipld.Node
	blockstore blockstore.Blockstore
}

var _ exec.Storage = (*Storage)(nil)

// NewStorage creates a datastore backed storage object for the given actor
func NewStorage(bs blockstore.Blockstore, act *actor.Actor) Storage {
	return Storage{
		chunks:     map[cid.Cid]ipld.Node{},
		actor:      act,
		blockstore: bs,
	}
}

// Put adds a node to temporary storage by id.
func (s Storage) Put(v interface{}) (cid.Cid, error) {
	var nd format.Node
	var err error
	if blk, ok := v.(blocks.Block); ok {
		// optimize putting blocks
		nd, err = cbor.DecodeBlock(blk)
	} else if bytes, ok := v.([]byte); ok {
		nd, err = cbor.Decode(bytes, types.DefaultHashFunction, -1)
	} else {
		nd, err = cbor.WrapObject(v, types.DefaultHashFunction, -1)
	}
	if err != nil {
		return cid.Undef, exec.Errors[exec.ErrDecode]
	}

	c := nd.Cid()
	s.chunks[c] = nd

	return c, nil
}

// Get retrieves a chunk from either temporary storage or its backing store.
// If the chunk is not found in storage, a vm.ErrNotFound error is returned.
func (s Storage) Get(cid cid.Cid) ([]byte, error) {
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
func (s Storage) Commit(newCid cid.Cid, oldCid cid.Cid) error {
	// commit to initialize actor only permitted if Head and expected id are nil
	if oldCid.Defined() && s.actor.Head.Defined() && !oldCid.Equals(s.actor.Head) {
		return exec.Errors[exec.ErrStaleHead]
	} else if oldCid != s.actor.Head { // covers case where only one cid is nil
		return exec.Errors[exec.ErrStaleHead]
	}

	// validate completeness by traversing graph to find ids
	if _, err := s.liveDescendantIds(newCid); err != nil {
		return exec.Errors[exec.ErrDanglingPointer]
	}

	s.actor.Head = newCid

	return nil
}

// Head return the current head of the actor's memory
func (s Storage) Head() cid.Cid {
	return s.actor.Head
}

// Prune removes all chunks that are unlinked
func (s *Storage) Prune() error {
	liveIds, err := s.liveDescendantIds(s.actor.Head)
	if err != nil {
		return err
	}

	if liveIds.Len() == len(s.chunks) {
		return nil
	}

	for id := range s.chunks {
		if !liveIds.Has(id) {
			delete(s.chunks, id)
		}
	}

	return nil
}

// Flush write storage to underlying datastore
func (s *Storage) Flush() error {
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

// liveDescendantIds returns the ids of all chunks reachable from the given id for this storage.
// That is the given id , any links in the chunk referenced by the given id, or any links
// referenced from those links.
func (s Storage) liveDescendantIds(id cid.Cid) (*cid.Set, error) {
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
