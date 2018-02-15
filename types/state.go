package types

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"
)

// StateTree is the in memory representation of the state tree of the chain.
type StateTree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

// LoadStateTree retrieves the state tree from a given store at the given position.
func LoadStateTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid) (*StateTree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, err
	}

	return &StateTree{
		root:  root,
		store: store,
	}, nil
}

// NewEmptyStateTree initializes a new state tree.
func NewEmptyStateTree(store *hamt.CborIpldStore) *StateTree {
	return &StateTree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

// Flush writes the current state of the state tree to the underlying store.
func (t *StateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

// GetActor retrieves an actor by their address.
// If no actor exists at the given address an error is returned.
func (t *StateTree) GetActor(ctx context.Context, a Address) (*Actor, error) {
	data, err := t.root.Find(ctx, string(a))
	if err != nil {
		return nil, err
	}

	var act Actor
	if err := act.Unmarshal(data); err != nil {
		return nil, err
	}

	return &act, nil
}

// GetOrCreateActor retrieves an actor by their address
// If no actor exists at the given address it returns a newly initialized actor.
func (t *StateTree) GetOrCreateActor(ctx context.Context, address Address) (*Actor, error) {
	act, err := t.GetActor(ctx, address)
	switch err {
	default:
		return nil, err
	case hamt.ErrNotFound:
		// TODO: better code default
		act = NewActor(AccountActorCid)
	case nil:
	}

	return act, nil
}

// SetActor stores the given actor at the specified address.
func (t *StateTree) SetActor(ctx context.Context, a Address, act *Actor) error {
	data, err := act.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}

	if err := t.root.Set(ctx, string(a), data); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}
