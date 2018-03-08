package types

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

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
	GetOrCreateActor(ctx context.Context, a Address, c func() (*Actor, error)) (*Actor, error)
	SetActor(ctx context.Context, a Address, act *Actor) error
}

var _ StateTree = &stateTree{}

// LoadStateTree loads the state tree referenced by the given cid.
func LoadStateTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid) (StateTree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node")
	}

	return &stateTree{
		root:  root,
		store: store,
	}, nil
}

// NewEmptyStateTree instantiates a new state tree with no data in it.
func NewEmptyStateTree(store *hamt.CborIpldStore) StateTree {
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

// SetActor sets the memory slot at address 'a' to the given actor.
// This operation can overwrite existing actors at that address.
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

// DebugStateTree prints a debug version of the current state tree.
func DebugStateTree(t StateTree) {
	st, ok := t.(*stateTree)
	if !ok {
		panic("can debug non stateTree")
	}
	st.debugPointer(st.root.Pointers)
}

func (t *stateTree) debugPointer(ps []*hamt.Pointer) {
	fmt.Println("---- state tree -- ")
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
