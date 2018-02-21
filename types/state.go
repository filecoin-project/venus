package types

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// stateTree is the in memory representation of the state tree of the chain.
type stateTree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

// StateTree is the interface that stateTree (the implementation) exports. This
// interface makes it easy to replace a StateTree with a mock or fake state tree
// when testing higher-level code.
type StateTree interface {
	Flush(ctx context.Context) (*cid.Cid, error)
	GetActor(ctx context.Context, a Address) (*Actor, error)
	GetOrCreateActor(ctx context.Context, a Address, c func() (*Actor, error)) (*Actor, error)
	SetActor(ctx context.Context, a Address, act *Actor) error
}

var _ StateTree = &stateTree{}

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

// NewEmptyStateTree initializes a new state tree.
func NewEmptyStateTree(store *hamt.CborIpldStore) *stateTree {
	return &stateTree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

// Flush writes the current state of the state tree to the underlying store.
func (t *stateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

// GetActor retrieves an actor by their address.
// If no actor exists at the given address an error is returned.
func (t *stateTree) GetActor(ctx context.Context, a Address) (*Actor, error) {
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
func (t *stateTree) GetOrCreateActor(ctx context.Context, address Address, creator func() (*Actor, error)) (*Actor, error) {
	act, err := t.GetActor(ctx, address)
	switch err {
	default:
		return nil, err
	case hamt.ErrNotFound:
		return creator()
	case nil:
	}

	return act, nil
}

// SetActor stores the given actor at the specified address.
func (t *stateTree) SetActor(ctx context.Context, a Address, act *Actor) error {
	data, err := act.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}

	if err := t.root.Set(ctx, string(a), data); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}

// Debug prints a debug version of the current state tree.
func (t *stateTree) Debug() {
	t.debugPointer(t.root.Pointers)
}

func (t *stateTree) debugPointer(ps []*hamt.Pointer) {
	for _, p := range ps {
		fmt.Println("----")
		for _, kv := range p.KVs {
			fmt.Printf("%s: %X\n", kv.Key, kv.Value)
		}
		if p.Link != nil {
			n, err := hamt.LoadNode(context.Background(), t.store, p.Link)
			if err != nil {
				fmt.Printf("unable to print link: %s: %s\n", p.Link.String(), err)
				continue
			}

			t.debugPointer(n.Pointers)
		}
	}
}
