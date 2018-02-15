package types

import (
	"fmt"

	atlas "gx/ipfs/QmSaDQWMxJBMtzQWnGoDppbwSEbHv4aJcD86CMSdszPU4L/refmt/obj/atlas"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(actorCborEntry)
}

var (
	// ErrInvalidActorLength is returned when the actor length does not match the expected length.
	ErrInvalidActorLength = errors.New("invalid actor length")
)

// Actor is the central abstraction of entities in the system.
// TODO: write better docs
type Actor struct {
	code   *cid.Cid
	memory []byte
	nonce  uint64
}

// Code retrieves the code of this actor.
func (a *Actor) Code() *cid.Cid {
	return a.code
}

// Nonce retrieves the nonce of this actor.
func (a *Actor) Nonce() uint64 {
	return a.nonce
}

// IncNonce increments the nonce of this actor by 1.
func (a *Actor) IncNonce() {
	a.nonce = a.nonce + 1
}

// ReadStorage retrieves all storage of this actor.
func (a *Actor) ReadStorage() []byte {
	out := make([]byte, len(a.memory))
	copy(out, a.memory)
	return out
}

// WriteStorage sets the storage of this actor.
// All existing storage is overwritten.
func (a *Actor) WriteStorage(memory []byte) {
	if len(a.memory) < len(memory) {
		// Grow memory as needed for now
		a.memory = make([]byte, len(memory))
	} else if len(a.memory) > len(memory) {
		// Shrink memory down
		a.memory = a.memory[0:len(memory)]
	}
	copy(a.memory, memory)
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
func NewActor(code *cid.Cid) *Actor {
	return &Actor{
		code:   code,
		memory: []byte{},
		nonce:  0,
	}
}

// NewActorWithMemory constructs a new actor with a predefined memory.
func NewActorWithMemory(code *cid.Cid, memory []byte) *Actor {
	return &Actor{
		code:   code,
		memory: memory,
		nonce:  0,
	}
}

var actorCborEntry = atlas.
	BuildEntry(Actor{}).
	Transform().
	TransformMarshal(atlas.MakeMarshalTransformFunc(marshalActor)).
	TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(unmarshalActor)).
	Complete()

func marshalActor(actor Actor) ([]interface{}, error) {
	return []interface{}{
		actor.code.Bytes(),
		actor.memory,
		actor.nonce,
	}, nil
}

func unmarshalActor(x []interface{}) (Actor, error) {
	if len(x) != 3 {
		return Actor{}, ErrInvalidActorLength
	}

	rawCode, ok := x[0].([]byte)
	if !ok {
		return Actor{}, errInvalidActor("code", x[0])
	}

	code, err := cid.Cast(rawCode)
	if err != nil {
		return Actor{}, errors.Wrapf(err, "invalid code cid: %v", rawCode)
	}

	memory, ok := x[1].([]byte)
	if !ok {
		return Actor{}, errInvalidActor("memory", x[1])
	}

	nonce, ok := x[2].(uint64)
	if !ok && x[2] != nil {
		return Actor{}, errInvalidActor("nonce", x[2])
	}

	return Actor{
		code:   code,
		memory: memory,
		nonce:  nonce,
	}, nil
}

// Unmarshal a actor from the given bytes.
func (a *Actor) Unmarshal(b []byte) error {
	return cbor.DecodeInto(b, a)
}

// Marshal the actor into bytes.
func (a *Actor) Marshal() ([]byte, error) {
	return cbor.DumpObject(a)
}

func errInvalidActor(field string, received interface{}) error {
	return fmt.Errorf("invalid actor %s field: %v", field, received)
}
