package types

import (
	"fmt"
	cbor "gx/ipfs/QmSyK1ZiAP98YvnxsTfQpb669V2xeTHRbG4Y6fgKS3vVSd/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
)

func init() {
	cbor.RegisterCborType(Actor{})
}

var (
	// ErrInvalidActorLength is returned when the actor length does not match the expected length.
	ErrInvalidActorLength = errors.New("invalid actor length")
)

// Actor is the central abstraction of entities in the system.
//
// Both individual accounts, as well as contracts (user & system level) are
// represented as actors. An actor has the following core functionality implemented on a system level:
// - track a Filecoin balance, using the `Balance` field
// - execute code stored in the `Code` field
// - read & write memory
// - replay protection, using the `Nonce` field
//
// Value sent to a non-existent address will be tracked as an empty actor that has a Balance but
// nil Code and Memory. You must nil check Code cids before comparing them.
//
// More specific capabilities for individual accounts or contract specific must be implemented
// inside the code.
//
// Not safe for concurrent access.
type Actor struct {
	Code    *cid.Cid
	Head    *cid.Cid
	Memory  []byte
	Nonce   Uint64
	Balance *AttoFIL
}

// IncNonce increments the nonce of this actor by 1.
func (a *Actor) IncNonce() {
	a.Nonce = a.Nonce + 1
}

// ReadStorage returns a copy of the actor's storage.
func (a *Actor) ReadStorage() []byte {
	out := make([]byte, len(a.Memory))
	copy(out, a.Memory)
	return out
}

// WriteStorage sets the storage of this actor.
// All existing storage is overwritten.
func (a *Actor) WriteStorage(memory []byte) {
	if len(a.Memory) < len(memory) {
		// Grow memory as needed for now
		a.Memory = make([]byte, len(memory))
	} else if len(a.Memory) > len(memory) {
		// Shrink memory down
		a.Memory = a.Memory[0:len(memory)]
	}
	copy(a.Memory, memory)
}

// Cid returns the canonical CID for the actor.
// TODO: can we avoid returning an error?
func (a *Actor) Cid() (*cid.Cid, error) {
	obj, err := cbor.WrapObject(a, DefaultHashFunction, -1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// NewActor constructs a new actor.
func NewActor(code *cid.Cid, balance *AttoFIL) *Actor {
	return &Actor{
		Code:    code,
		Memory:  []byte{},
		Nonce:   0,
		Balance: balance,
	}
}

// NewActorWithMemory constructs a new actor with a predefined memory.
func NewActorWithMemory(code *cid.Cid, balance *AttoFIL, memory []byte) *Actor {
	return &Actor{
		Code:    code,
		Memory:  memory,
		Nonce:   0,
		Balance: balance,
	}
}

// Unmarshal a actor from the given bytes.
func (a *Actor) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, a)
}

// Marshal the actor into bytes.
func (a *Actor) Marshal() ([]byte, error) {
	return cbor.DumpObject(a)
}

// Format implements fmt.Formatter.
func (a *Actor) Format(f fmt.State, c rune) {
	f.Write([]byte(fmt.Sprintf("<%s (%p); balance: %v; nonce: %d>", ActorCodeTypeName(a.Code), a, a.Balance, a.Nonce))) // nolint: errcheck
}
