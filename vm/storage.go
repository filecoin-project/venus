package vm

import (
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"

	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// StorageMap holds all staged memory for all actors.
type StorageMap struct {
	datastore  datastore.Datastore
	storageMap map[types.Address]Storage
}

// NewStorageMap returns a storage object for the given datastore.
func NewStorageMap(ds datastore.Datastore) *StorageMap {
	return &StorageMap{
		datastore:  ds,
		storageMap: map[types.Address]Storage{},
	}
}

// NewStorage gets or creates a Storage for the given address
func (s StorageMap) NewStorage(addr types.Address, actor *types.Actor) Storage {
	stage, ok := s.storageMap[addr]
	if ok {
		// Return a hybrid stage with the pre-existing chunk, but the given instance of the actor.
		// This ensures changes made to the actor appear in the state tree cache.
		return Storage{
			actor:     actor,
			chunks:    stage.chunks,
			datastore: s.datastore,
		}
	}

	stage = Storage{
		chunks:    map[string]ipld.Node{},
		actor:     actor,
		datastore: s.datastore,
	}
	s.storageMap[addr] = stage

	// Temporary pending permanent storage
	head, _ := stage.Put(actor.Memory) // ignore errors for now
	actor.Head = head

	// Temporary pending permanent storage
	head, _ = stage.Put(actor.Memory) // ignore errors for now
	actor.Head = head

	// Temporary pending permanent storage
	head, _ = stage.Put(actor.Memory) // ignore errors for now
	actor.Head = head

	return stage
}

// Flush saves all valid staged changes to the datastore
func (s StorageMap) Flush() error {
	for _, storage := range s.storageMap {
		liveChunks, err := storage.liveIds()
		if err != nil {
			return err
		}

		for idKey := range liveChunks {
			chunk := storage.chunks[idKey]
			key := datastore.NewKey(idKey)
			err = s.datastore.Put(key, chunk.RawData())
			if err != nil {
				return errors.RevertErrorWrapf(err, "Errors storing data chunk")
			}
		}
	}

	return nil
}

// Storage is a place to hold chunks that are created while processing a block.
type Storage struct {
	actor     *types.Actor
	chunks    map[string]ipld.Node
	datastore datastore.Datastore
}

// Put adds a node to temporary storage by id
func (s Storage) Put(chunk []byte) (*cid.Cid, exec.ErrorCode) {
	n, err := cbor.Decode(chunk, types.DefaultHashFunction, -1)
	if err != nil {
		return nil, exec.ErrDecode
	}

	cid := n.Cid()
	s.chunks[cid.KeyString()] = n
	return cid, exec.Ok
}

// Get retrieves a node from either temporary storage or its backing store.
// The returned bool indicates whether or not the object was found. The error
// indicates and error fetching from storage.
func (s Storage) Get(cid *cid.Cid) ([]byte, bool, error) {
	key := cid.KeyString()
	n, ok := s.chunks[key]
	if ok {
		return n.RawData(), ok, nil
	}

	chunk, err := s.datastore.Get(datastore.NewKey(key))
	if err != nil {
		if err == datastore.ErrNotFound {
			return []byte{}, false, nil
		}
		return []byte{}, false, err
	}

	chunkBytes, ok := chunk.([]byte)
	if !ok {
		return []byte{}, false, errors.NewFaultError("Invalid format for retrieved chunk")
	}

	return chunkBytes, true, nil
}

// Commit stores the head of the given actor.
// The new cid must have been put in the stage.
// The given oldCid must match the current cid.
func (s Storage) Commit(newCid *cid.Cid, oldCid *cid.Cid) exec.ErrorCode {
	if !oldCid.Equals(s.actor.Head) {
		return exec.ErrStaleHead
	}

	chunk, ok := s.chunks[newCid.KeyString()]
	if !ok {
		return exec.ErrDanglingPointer
	}

	// validate completeness by traversing graph to find ids
	if _, err := s.liveDescendantIds(newCid); err != nil {
		return exec.ErrDanglingPointer
	}

	s.actor.Head = newCid

	// TODO: remove the actor memory property when we have persistent storage
	s.actor.Memory = chunk.RawData()

	return exec.Ok
}

// Head return the current head of the actor's memory
func (s Storage) Head() *cid.Cid {
	return s.actor.Head
}

// Prune removes all chunks that are unlinked
func (s *Storage) Prune() error {
	liveIds, err := s.liveIds()
	if err != nil {
		return err
	}

	if len(liveIds) == len(s.chunks) {
		return nil
	}

	for idKey := range s.chunks {
		_, ok := liveIds[idKey]
		if !ok {
			delete(s.chunks, idKey)
		}
	}

	return nil
}

func (s Storage) liveIds() (map[string]*cid.Cid, error) {
	return s.liveDescendantIds(s.actor.Head)
}

func (s Storage) liveDescendantIds(id *cid.Cid) (map[string]*cid.Cid, error) {
	chunk, ok := s.chunks[id.KeyString()]
	if !ok {
		has, err := s.datastore.Has(datastore.NewKey(id.KeyString()))
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "linked node, %s, missing from stage during flush", id)
		}

		// unstaged chunk that exists in datastore is valid, but halts recursion.
		if has {
			return map[string]*cid.Cid{}, nil
		}

		return nil, errors.NewFaultErrorf("linked node, %s, missing from stage during flush", id)
	}

	ids := map[string]*cid.Cid{id.KeyString(): id}

	for _, link := range chunk.Links() {
		linked, err := s.liveDescendantIds(link.Cid)
		if err != nil {
			return nil, err
		}
		for idKey, id := range linked {
			ids[idKey] = id
		}
	}

	return ids, nil
}
