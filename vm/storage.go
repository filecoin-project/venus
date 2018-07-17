package vm

import (
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// StorageMap holds all staged memory for all actors.
type StorageMap map[types.Address]Storage

// NewStorage gets or creates a Storage for the given address
func (s StorageMap) NewStorage(addr types.Address, actor *types.Actor) Storage {
	stage, ok := s[addr]
	if ok {
		// Return a hybrid stage with the pre-existing chunk, but the given instance of the actor.
		// This ensures changes made to the actor appear in the state tree cache.
		return Storage{
			actor:  actor,
			chunks: stage.chunks,
		}
	}

	stage = Storage{
		chunks: map[string]ipld.Node{},
		actor:  actor,
	}
	s[addr] = stage

	// Temporary pending permanent storage
	head, _ := stage.Put(actor.Memory) // ignore errors for now
	actor.Head = head

	return stage
}

// Storage is a place to hold chunks that are created while processing a block.
type Storage struct {
	actor  *types.Actor
	chunks map[string]ipld.Node
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

// Get retrieves a node from temporary storage.
func (s Storage) Get(cid *cid.Cid) ([]byte, bool) {
	n, ok := s.chunks[cid.KeyString()]
	if ok {
		return n.RawData(), ok
	}

	return nil, ok
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

	s.actor.Head = newCid

	// TODO: remove the actor memory property when we have persistent storage
	s.actor.Memory = chunk.RawData()

	return exec.Ok
}

// Head return the current head of the actor's memory
func (s Storage) Head() *cid.Cid {
	return s.actor.Head
}
