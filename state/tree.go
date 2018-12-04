package state

import (
	"context"
	"fmt"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRXf2uUSdGSunRJsM9wXSUNVwLUGCY3So5fAs7h2CBJVf/go-hamt-ipld"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmfWqohMtbivn5NRJvtrLzCW3EU4QmoLvVNtmvo9vbdtVA/refmt/obj"
	"gx/ipfs/QmfWqohMtbivn5NRJvtrLzCW3EU4QmoLvVNtmvo9vbdtVA/refmt/shared"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
)

// tree is a state tree that maps addresses to actors.
type tree struct {
	// root is the root of the state merklehamt
	root  *hamt.Node
	store *hamt.CborIpldStore

	builtinActors map[string]exec.ExecutableActor
}

// RevID identifies a snapshot of the StateTree.
type RevID int

// ActorWalkFn is a visitor function for actors
type ActorWalkFn func(address.Address, *actor.Actor) error

// Tree is the interface that stateTree implements. It provides accessors
// to Get and Set actors in a backing store by address.
type Tree interface {
	Flush(ctx context.Context) (*cid.Cid, error)

	GetActor(ctx context.Context, a address.Address) (*actor.Actor, error)
	GetOrCreateActor(ctx context.Context, a address.Address, c func() (*actor.Actor, error)) (*actor.Actor, error)
	SetActor(ctx context.Context, a address.Address, act *actor.Actor) error

	ForEachActor(ctx context.Context, walkFn ActorWalkFn) error

	GetBuiltinActorCode(c *cid.Cid) (exec.ExecutableActor, error)
}

var _ Tree = &tree{}

// LoadStateTree loads the state tree referenced by the given cid.
func LoadStateTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid, builtinActors map[string]exec.ExecutableActor) (Tree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node")
	}
	stateTree := newEmptyStateTree(store)
	stateTree.root = root

	stateTree.builtinActors = builtinActors

	return stateTree, nil
}

// NewEmptyStateTree instantiates a new state tree with no data in it.
func NewEmptyStateTree(store *hamt.CborIpldStore) Tree {
	return newEmptyStateTree(store)
}

// NewEmptyStateTreeWithActors instantiates a new state tree with no data in it, except for the passed in actors.
func NewEmptyStateTreeWithActors(store *hamt.CborIpldStore, builtinActors map[string]exec.ExecutableActor) Tree {
	s := newEmptyStateTree(store)
	s.builtinActors = builtinActors
	return s
}

func newEmptyStateTree(store *hamt.CborIpldStore) *tree {
	return &tree{
		root:          hamt.NewNode(store),
		store:         store,
		builtinActors: map[string]exec.ExecutableActor{},
	}
}

// Flush serialized the state tree and flushes unflushed changes to the backing
// datastore. The cid of the state tree is returned.
func (t *tree) Flush(ctx context.Context) (*cid.Cid, error) {
	if err := t.root.Flush(ctx); err != nil {
		return nil, err
	}

	return t.store.Put(ctx, t.root)
}

// IsActorNotFoundError is true of the error returned by
// GetActor when no actor was found at the given address.
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

func (t *tree) GetBuiltinActorCode(codePointer *cid.Cid) (exec.ExecutableActor, error) {
	if codePointer == nil {
		return nil, fmt.Errorf("missing code")
	}
	actor, ok := t.builtinActors[codePointer.KeyString()]
	if !ok {
		return nil, fmt.Errorf("unknown code: %s", codePointer.String())
	}

	return actor, nil
}

// GetActor retrieves an actor by their address. If no actor
// exists at the given address then an error will be returned
// for which IsActorNotFoundError(err) is true.
func (t *tree) GetActor(ctx context.Context, a address.Address) (*actor.Actor, error) {
	data, err := t.root.Find(ctx, a.String())
	if err == hamt.ErrNotFound {
		return nil, &actorNotFoundError{}
	} else if err != nil {
		return nil, err
	}

	var act actor.Actor
	if err := hackTransferObject(data, &act); err != nil {
		return nil, err
	}

	return &act, nil
}

func hackTransferObject(from, to interface{}) error {
	m := obj.NewMarshaller(cbor.CborAtlas)
	if err := m.Bind(from); err != nil {
		return err
	}

	u := obj.NewUnmarshaller(cbor.CborAtlas)
	if err := u.Bind(to); err != nil {
		return err
	}

	return shared.TokenPump{
		TokenSource: m,
		TokenSink:   u,
	}.Run()
}

// GetOrCreateActor retrieves an actor by their address
// If no actor exists at the given address it returns a newly initialized actor.
func (t *tree) GetOrCreateActor(ctx context.Context, address address.Address, creator func() (*actor.Actor, error)) (*actor.Actor, error) {
	act, err := t.GetActor(ctx, address)
	if IsActorNotFoundError(err) {
		return creator()
	}
	return act, err
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

func forEachActor(ctx context.Context, cst *hamt.CborIpldStore, nd *hamt.Node, walkFn ActorWalkFn) error {
	for _, p := range nd.Pointers {
		for _, kv := range p.KVs {
			var a actor.Actor
			if err := hackTransferObject(kv.Value, &a); err != nil {
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
		if p.Link != nil {
			n, err := hamt.LoadNode(context.Background(), cst, p.Link)
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

// GetAllActors returns a slice of all actors in the StateTree, t.
func GetAllActors(t Tree) ([]string, []*actor.Actor) {
	st := t.(*tree)

	return st.getActorsFromPointers(st.root.Pointers)
}

// GetAllActorsFromStoreFunc is a function with the signature of GetAllActorsFromStore
type GetAllActorsFromStoreFunc = func(context.Context, *hamt.CborIpldStore, *cid.Cid) ([]string, []*actor.Actor, error)

// GetAllActorsFunc is a function with the signature of GetAllActors
type GetAllActorsFunc = func(t Tree) ([]string, []*actor.Actor)

// GetAllActorsFromStore loads a StateTree and returns arrays of addresses and their corresponding actors.
// Third returned value is any error that occurred when loading.
func GetAllActorsFromStore(ctx context.Context, store *hamt.CborIpldStore, stateRoot *cid.Cid) ([]string, []*actor.Actor, error) {
	st, err := LoadStateTree(ctx, store, stateRoot, nil)
	if err != nil {
		return nil, nil, err
	}

	addrs, actors := GetAllActors(st)
	return addrs, actors, nil
}

// NOTE: This extracts actors from pointers recursively. Maybe we shouldn't recurse here.
func (t *tree) getActorsFromPointers(ps []*hamt.Pointer) (addresses []string, actors []*actor.Actor) {
	for _, p := range ps {
		for _, kv := range p.KVs {
			var a actor.Actor
			if err := hackTransferObject(kv.Value, &a); err != nil {
				panic(err) // uhm, ignoring errors is bad
			}

			addresses = append(addresses, kv.Key)
			actors = append(actors, &a)
		}
		if p.Link != nil {
			n, err := hamt.LoadNode(context.Background(), t.store, p.Link)
			// Even if we hit an error and can't follow this link, we should
			// keep traversing its siblings.
			if err != nil {
				continue
			}
			moreAddrs, moreActors := t.getActorsFromPointers(n.Pointers)
			addresses = append(addresses, moreAddrs...)
			actors = append(actors, moreActors...)
		}
	}
	return
}
