// Package actor implements tooling to write and manipulate actors in go.
package actor

import (
	"context"
	"fmt"

	"gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	block "gx/ipfs/QmWAzSEoqZ6xU6pu8yL8e5WaMb7wtbfbhhN4p1DknUPtr3/go-block-format"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	vmerrors "github.com/filecoin-project/go-filecoin/vm/errors"
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
	Nonce   types.Uint64
	Balance *types.AttoFIL
}

// IncNonce increments the nonce of this actor by 1.
func (a *Actor) IncNonce() {
	a.Nonce = a.Nonce + 1
}

// Cid returns the canonical CID for the actor.
// TODO: can we avoid returning an error?
func (a *Actor) Cid() (*cid.Cid, error) {
	obj, err := cbor.WrapObject(a, types.DefaultHashFunction, -1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal to cbor")
	}

	return obj.Cid(), nil
}

// NewActor constructs a new actor.
func NewActor(code *cid.Cid, balance *types.AttoFIL) *Actor {
	return &Actor{
		Code:    code,
		Head:    nil,
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
	f.Write([]byte(fmt.Sprintf("<%s (%p); balance: %v; nonce: %d>", types.ActorCodeTypeName(a.Code), a, a.Balance, a.Nonce))) // nolint: errcheck
}

// SetKeyValue convenience method to load a lookup, set one key value pair and commit.
// This function is inefficient when multiple values need to be set into the lookup.
func SetKeyValue(ctx context.Context, storage exec.Storage, id *cid.Cid, key string, value interface{}) (*cid.Cid, error) {
	lookup, err := LoadLookup(ctx, storage, id)
	if err != nil {
		return nil, err
	}

	err = lookup.Set(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return lookup.Commit(ctx)
}

// LoadLookup loads hamt-ipld node from storage if the cid exists, or creates a new on if it is nil.
// The lookup provides access to a HAMT/CHAMP tree stored in storage.
func LoadLookup(ctx context.Context, storage exec.Storage, cid *cid.Cid) (exec.Lookup, error) {
	cborStore := &hamt.CborIpldStore{
		Blocks: &storageAsBlocks{s: storage},
		Atlas:  &cbor.CborAtlas,
	}
	var root *hamt.Node
	var err error

	if cid == nil {
		root = hamt.NewNode(cborStore)
	} else {
		root, err = hamt.LoadNode(ctx, cborStore, cid)
		if err != nil {
			return nil, err
		}
	}

	return &lookup{n: root, s: storage}, nil
}

// storageAsBlocks allows us to use an exec.Storage as a Blockstore
type storageAsBlocks struct {
	s exec.Storage
}

// GetBlock gets a block from underlying storage by cid
func (sab *storageAsBlocks) GetBlock(ctx context.Context, c *cid.Cid) (block.Block, error) {
	chunk, err := sab.s.Get(c)
	if err != nil {
		return nil, err
	}

	return block.NewBlock(chunk), nil
}

// AddBlock add a block to underlying storage
func (sab *storageAsBlocks) AddBlock(b block.Block) error {
	_, err := sab.s.Put(b.RawData())
	return err
}

// lookup implements exec.Lookup and provides structured key-value storage for actors
type lookup struct {
	n *hamt.Node
	s exec.Storage
}

var _ exec.Lookup = (*lookup)(nil)

// Find retrieves a value by key
func (l *lookup) Find(ctx context.Context, k string) (interface{}, error) {
	return l.n.Find(ctx, k)
}

// Set adds a value under the given key
func (l *lookup) Set(ctx context.Context, k string, v interface{}) error {
	return l.n.Set(ctx, k, v)
}

// Commit ensures all data in the tree is flushed to storage and returns the cid of the head node.
func (l *lookup) Commit(ctx context.Context) (*cid.Cid, error) {
	if err := l.n.Flush(ctx); err != nil {
		return nil, err
	}

	chunk, err := cbor.DumpObject(l.n)
	if err != nil {
		return nil, err
	}

	return l.s.Put(chunk)
}

// Values returns a slice of all key-values stored in the lookup
func (l *lookup) Values(ctx context.Context) ([]*hamt.KV, error) {
	return l.values(ctx, []*hamt.KV{})
}

func (l *lookup) values(ctx context.Context, vs []*hamt.KV) ([]*hamt.KV, error) {
	for _, p := range l.n.Pointers {
		vs = append(vs, p.KVs...)

		if p.Link == nil {
			continue
		}

		subtree, err := LoadLookup(ctx, l.s, p.Link)
		if err != nil {
			return nil, err
		}

		sublookup, ok := subtree.(*lookup)
		if !ok {
			return nil, vmerrors.NewFaultError("Non-actor.lookup found in hamt tree")
		}

		vs, err = sublookup.values(ctx, vs)
		if err != nil {
			return nil, err
		}
	}
	return vs, nil
}
