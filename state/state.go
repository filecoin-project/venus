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

type Tree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	store *hamt.CborIpldStore
}

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

func NewEmptyTree(store *hamt.CborIpldStore) *Tree {
	return &Tree{
		root:  hamt.NewNode(store),
		store: store,
	}
}

func (t *Tree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

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
