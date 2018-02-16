package types

import (
	"context"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"
)

func init() {
	cbor.RegisterCborType(Actor{})
}

type StateTree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

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

func NewEmptyTree(store *hamt.CborIpldStore) *StateTree {
	return &StateTree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

func (t *StateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

func (t *StateTree) GetActor(ctx context.Context, a Address) (*Actor, error) {
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

func (t *StateTree) SetActor(ctx context.Context, a Address, act *Actor) error {
	data, err := cbor.DumpObject(act)
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}
	if err := t.root.Set(ctx, string(a), data); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}
