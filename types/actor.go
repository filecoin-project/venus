package types

import (
	"math/big"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(Actor{})
}

var (
	// ErrInvalidActorLength is returned when the actor length does not match the expected length.
	ErrInvalidActorLength = errors.New("invalid actor length")
)

// Actor is the central abstraction of entities in the system.
// TODO: write better docs
type Actor struct {
	Code    *cid.Cid
	Memory  []byte
	Nonce   uint64
	Balance *big.Int
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
func NewActor(code *cid.Cid, balance *big.Int) *Actor {
	return &Actor{
		Code:    code,
		Memory:  []byte{},
		Nonce:   0,
		Balance: balance,
	}
}

// NewActorWithMemory constructs a new actor with a predefined memory.
func NewActorWithMemory(code *cid.Cid, balance *big.Int, memory []byte) *Actor {
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
