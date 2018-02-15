// Package state implements a merkelized blockchain state tree.
package state

import (
	"context"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cbor "gx/ipfs/QmZpue627xQuNGXn7xHieSjSZ8N4jot6oBHwe9XTn3e4NU/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmdBXcN47jVwKLwSyN9e9xYVZ7WcAWgQ5N4cmNw7nzWq2q/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(Actor{})
}

// Tree is a state tree that maps addresses to actors.
type Tree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

// LoadTree loads the state tree referenced by the given cid.
func LoadTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid) (*Tree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, err
	}

	return &Tree{
		root:  root,
		store: store,
	}, nil
}

// NewEmptyTree instantiates a new state tree with no data in it.
func NewEmptyTree(store *hamt.CborIpldStore) *Tree {
	return &Tree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

// Flush serialized the state tree and flushes unflushed changes to the backing
// datastore. The cid of the state tree is returned.
func (t *Tree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

// GetActor looks up the actor with address 'a'.
func (t *Tree) GetActor(ctx context.Context, a types.Address) (*Actor, error) {
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
func (t *Tree) SetActor(ctx context.Context, a types.Address, act *Actor) error {
	data, err := cbor.DumpObject(act)
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}
	if err := t.root.Set(ctx, string(a), data); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}

type Actor struct {
	Code    *cid.Cid
	Memory  *cid.Cid
	Balance *big.Int
	Nonce   uint64
}
