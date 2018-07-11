package vm

import (
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"

	cbor "gx/ipfs/QmRiRJhn427YVuufBEHofLreKWNw7P7BWNq86Sb9kzqdbd/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

// Storage holds all staged memory for all actors.
type Storage map[types.Address]Stage

// NewStage gets or creates a Stage for the given address.
func (s Storage) NewStage(addr types.Address) Stage {
	stage, ok := s[addr]
	if ok {
		return stage
	}

	stage = Stage{}
	s[addr] = stage

	return stage
}

// Stage is a place to hold chunks that are created while processing a block.
type Stage map[string]ipld.Node

// Put adds a node to temporary storage by id
func (s Stage) Put(chunk []byte) (*cid.Cid, exec.ErrorCode) {
	n, err := cbor.Decode(chunk, types.DefaultHashFunction, -1)
	if err != nil {
		return nil, exec.ErrDecode
	}

	cid := n.Cid()
	s[cid.KeyString()] = n
	return cid, exec.Ok
}

// Get retrieves a node from either temporary storage or its backing store.
func (s Stage) Get(cid *cid.Cid) ([]byte, bool) {
	n, ok := s[cid.KeyString()]
	if ok {
		return n.RawData(), ok
	}

	return nil, ok
}
