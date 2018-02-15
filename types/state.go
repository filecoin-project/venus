package types

import (
	"context"

	cbor "gx/ipfs/QmRVSCwQtW1rjHCay9NqKXDwbtKTgDcN4iY7PrpSqfKM5D/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

func init() {
	cbor.RegisterCborType(Actor{})
}

// stateTree is a state tree that maps addresses to actors.
type stateTree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

// StateTree is the interface that stateTree implements. It provides accessors
// to Get and Set actors in a backing store by address.
type StateTree interface {
	Flush(ctx context.Context) (*cid.Cid, error)
	GetActor(ctx context.Context, a Address) (*Actor, error)
	SetActor(ctx context.Context, a Address, act *Actor) error
}

var _ StateTree = &stateTree{}

// LoadStateTree loads the state tree referenced by the given cid.
func LoadStateTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid) (*stateTree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, err
	}

	return &stateTree{
		root:  root,
		store: store,
	}, nil
}

// NewEmptyStateTree instantiates a new state tree with no data in it.
func NewEmptyTree(store *hamt.CborIpldStore) *stateTree {
	return &stateTree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

// Flush serialized the state tree and flushes unflushed changes to the backing
// datastore. The cid of the state tree is returned.
func (t *stateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

// GetActor looks up the actor with address 'a'.
func (t *stateTree) GetActor(ctx context.Context, a Address) (*Actor, error) {
	data, err := t.root.Find(ctx, string(a))
	if err != nil {
		return nil, err
	}
	var act Actor
	if err := cbor.DecodeInto(data, &act); err != nil {
		return nil, err
	}
	return &act, nil
}

// SetActor sets the memory slot at address 'a' to the given actor.
// This operation can overwrite existing actors at that address.
func (t *stateTree) SetActor(ctx context.Context, a Address, act *Actor) error {
	data, err := cbor.DumpObject(act)
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}
	if err := t.root.Set(ctx, string(a), data); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}
