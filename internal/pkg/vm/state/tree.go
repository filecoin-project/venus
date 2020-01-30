package state

import (
	"context"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// Tree is the interface that stateTree implements. It provides accessors
// to Get and Set actors in a backing store by address.
type Tree interface {
	Flush(ctx context.Context) (cid.Cid, error)

	GetActor(ctx context.Context, a address.Address) (*actor.Actor, error)
	GetOrCreateActor(ctx context.Context, a address.Address, c func() (*actor.Actor, address.Address, error)) (*actor.Actor, address.Address, error)
	SetActor(ctx context.Context, a address.Address, act *actor.Actor) error

	ForEachActor(ctx context.Context, walkFn ActorWalkFn) error
	GetAllActors(ctx context.Context) <-chan GetAllActorsResult
}

// ActorWalkFn is a visitor function for actors
type ActorWalkFn func(address.Address, *actor.Actor) error

// GetAllActorsResult is the struct returned via a channel by the GetAllActors
// method. This struct contains only an address string and the actor itself.
type GetAllActorsResult struct {
	Address string
	Actor   *actor.Actor
	Error   error
}

// tree is a state tree that maps addresses to actors.
type tree struct {
	// root is the root of the state merklehamt
	root  *hamt.Node
	store hamt.CborIpldStore
}

const (
	// TreeBitWidth is the bit width of the HAMT used to store a state tree
	TreeBitWidth = 5
)

// NewTree instantiates a new state tree.
func NewTree(store hamt.CborIpldStore) Tree {
	return newEmptyStateTree(store)
}

func newEmptyStateTree(store hamt.CborIpldStore) *tree {
	return &tree{
		root:  hamt.NewNode(store, hamt.UseTreeBitWidth(TreeBitWidth)),
		store: store,
	}
}

var _ Tree = &tree{}

// Flush serialized the state tree and flushes unflushed changes to the backing
// datastore. The cid of the state tree is returned.
func (t *tree) Flush(ctx context.Context) (cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return cid.Undef, err
	}

	return t.store.Put(ctx, t.root)
}

// GetActorCode retrieves an actor by their address. If no actor
// exists at the given address then an error will be returned
// for which IsActorNotFoundError(err) is true.
func (t *tree) GetActor(ctx context.Context, a address.Address) (*actor.Actor, error) {
	var act actor.Actor
	err := t.root.Find(ctx, a.String(), &act)
	if err == hamt.ErrNotFound {
		return nil, &actorNotFoundError{}
	} else if err != nil {
		return nil, err
	}

	return &act, nil
}

// GetOrCreateActor retrieves an actor by their address
// If no actor exists at the given address it returns a newly initialized actor.
func (t *tree) GetOrCreateActor(ctx context.Context, addr address.Address, creator func() (*actor.Actor, address.Address, error)) (*actor.Actor, address.Address, error) {
	act, err := t.GetActor(ctx, addr)
	if IsActorNotFoundError(err) {
		return creator()
	}
	return act, addr, err
}

// SetActor sets the memory slot at address 'a' to the given actor.
// This operation can overwrite existing actors at that address.
func (t *tree) SetActor(ctx context.Context, a address.Address, act *actor.Actor) error {
	if err := t.root.Set(ctx, a.String(), act); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	return nil
}

// ForEachActor calls walkFn for each actor in the state tree
func (t *tree) ForEachActor(ctx context.Context, walkFn ActorWalkFn) error {
	return forEachActor(ctx, t.store, t.root, walkFn)
}

// GetAllActors returns a channel which provides all actors in the StateTree, t.
func (t *tree) GetAllActors(ctx context.Context) <-chan GetAllActorsResult {
	out := make(chan GetAllActorsResult)
	go func() {
		defer close(out)
		t.getActorsFromPointers(ctx, out, t.root.Pointers)
	}()
	return out
}

// IsActorNotFoundError is true of the error returned by
// GetActorCode when no actor was found at the given address.
func IsActorNotFoundError(err error) bool {
	cause := errors.Cause(err)
	e, ok := cause.(actornotfound)
	return ok && e.ActorNotFound()
}

type actornotfound interface {
	ActorNotFound() bool
}

type actorNotFoundError struct{}

func (e actorNotFoundError) Error() string {
	return "actor not found"
}

func (e actorNotFoundError) ActorNotFound() bool {
	return true
}

func forEachActor(ctx context.Context, cst hamt.CborIpldStore, nd *hamt.Node, walkFn ActorWalkFn) error {
	for _, p := range nd.Pointers {
		for _, kv := range p.KVs {
			var a actor.Actor
			if err := encoding.Decode(kv.Value.Raw, &a); err != nil {
				return err
			}

			addr, err := address.NewFromString(kv.Key)
			if err != nil {
				return err
			}

			if err := walkFn(addr, &a); err != nil {
				return err
			}
		}
		if p.Link.Defined() {
			n, err := hamt.LoadNode(context.Background(), cst, p.Link, hamt.UseTreeBitWidth(TreeBitWidth))
			if err != nil {
				return err
			}
			if err := forEachActor(ctx, cst, n, walkFn); err != nil {
				return err
			}
		}
	}
	return nil
}

// DebugStateTree prints a debug version of the current state tree.
func DebugStateTree(t Tree) {
	st, ok := t.(*tree)
	if !ok {
		panic("can't debug non-stateTree")
	}
	st.debugPointer(st.root.Pointers)
}

func (t *tree) debugPointer(ps []*hamt.Pointer) {
	fmt.Println("---- state tree -- ")
	for _, p := range ps {
		fmt.Println("----")
		for _, kv := range p.KVs {
			fmt.Printf("%s: %X\n", kv.Key, kv.Value)
		}
		if p.Link.Defined() {
			n, err := hamt.LoadNode(context.Background(), t.store, p.Link, hamt.UseTreeBitWidth(TreeBitWidth))
			if err != nil {
				fmt.Printf("unable to print link: %s: %s\n", p.Link.String(), err)
				continue
			}

			t.debugPointer(n.Pointers)
		}
	}
}

// NOTE: This extracts actors from pointers recursively. Maybe we shouldn't recurse here.
func (t *tree) getActorsFromPointers(ctx context.Context, out chan<- GetAllActorsResult, ps []*hamt.Pointer) {
	for _, p := range ps {
		for _, kv := range p.KVs {
			var a actor.Actor
			if err := encoding.Decode(kv.Value.Raw, &a); err != nil {
				panic(err) // uhm, ignoring errors is bad
			}

			select {
			case <-ctx.Done():
				out <- GetAllActorsResult{
					Error: ctx.Err(),
				}
				return
			default:
				out <- GetAllActorsResult{
					Address: kv.Key,
					Actor:   &a,
				}
			}
		}
		if p.Link.Defined() {
			n, err := hamt.LoadNode(context.Background(), t.store, p.Link, hamt.UseTreeBitWidth(TreeBitWidth))
			// Even if we hit an error and can't follow this link, we should
			// keep traversing its siblings.
			if err != nil {
				continue
			}
			t.getActorsFromPointers(ctx, out, n.Pointers)
		}
	}
}
