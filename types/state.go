package types

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	hamt "gx/ipfs/QmdtiofXbibTe6Day9ii5zjBZpSRm8vhfoerrNuY3sAQ7e/go-hamt-ipld"
)

// stateTree is a state tree that maps addresses to actors.
type stateTree struct {
	// root is the root of the state merklehamt
	root *hamt.Node

	// Snapshot-related fields. See comment on Snapshot().
	nextRevID RevID
	revs      map[RevID]*hamt.Node

	store *hamt.CborIpldStore
}

// RevID identifies a snapshot of the StateTree.
type RevID int

// StateTree is the interface that stateTree implements. It provides accessors
// to Get and Set actors in a backing store by address.
type StateTree interface {
	Flush(ctx context.Context) (*cid.Cid, error)

	GetActor(ctx context.Context, a Address) (*Actor, error)
	GetOrCreateActor(ctx context.Context, a Address, c func() (*Actor, error)) (*Actor, error)
	SetActor(ctx context.Context, a Address, act *Actor) error

	Snapshot() RevID
	RevertTo(RevID)
}

var _ StateTree = &stateTree{}

// LoadStateTree loads the state tree referenced by the given cid.
func LoadStateTree(ctx context.Context, store *hamt.CborIpldStore, c *cid.Cid) (StateTree, error) {
	root, err := hamt.LoadNode(ctx, store, c)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load node")
	}
	stateTree := newEmptyStateTree(store)
	stateTree.root = root
	return stateTree, nil
}

// NewEmptyStateTree instantiates a new state tree with no data in it.
func NewEmptyStateTree(store *hamt.CborIpldStore) StateTree {
	return newEmptyStateTree(store)
}

func newEmptyStateTree(store *hamt.CborIpldStore) *stateTree {
	return &stateTree{
		root:  hamt.NewNode(store),
		revs:  make(map[RevID]*hamt.Node),
		store: store,
	}
}

// Snapshot returns an identifier that can be used to revert to a
// previous state of the tree. Present implementation is quick and
// easy: we copy the underlying tree and keep it in a map by
// revid, then set it when RevertTo is called. This obviously keeps
// a full copy of the underlying tree around for each snapshot,
// forever. We should eventually do something better/different.
func (t *stateTree) Snapshot() RevID {
	thisRevID := t.nextRevID
	t.revs[thisRevID] = t.root.Copy()
	t.nextRevID++
	return thisRevID
}

// RevertTo reverts to the given RevID. You can revert to a given
// RevID multiple times.
func (t *stateTree) RevertTo(revID RevID) {
	root, ok := t.revs[revID]
	if !ok {
		panic("RevId does not exist")
	}
	// We have to return another copy here in case they roll back
	// to this state multiple times.
	t.root = root.Copy()
}

// Flush serialized the state tree and flushes unflushed changes to the backing
// datastore. The cid of the state tree is returned.
func (t *stateTree) Flush(ctx context.Context) (*cid.Cid, error) {
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

// GetActor retrieves an actor by their address. If no actor
// exists at the given address then an error will be returned
// for which IsActorNotFoundError(err) is true.
func (t *stateTree) GetActor(ctx context.Context, a Address) (*Actor, error) {
	data, err := t.root.Find(ctx, a.String())
	if err == hamt.ErrNotFound {
		return nil, &actorNotFoundError{}
	} else if err != nil {
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
	if IsActorNotFoundError(err) {
		return creator()
	}
	return act, err
}

// SetActor sets the memory slot at address 'a' to the given actor.
// This operation can overwrite existing actors at that address.
func (t *stateTree) SetActor(ctx context.Context, a Address, act *Actor) error {
	data, err := act.Marshal()
	if err != nil {
		return errors.Wrap(err, "marshal actor failed")
	}

	if err := t.root.Set(ctx, a.String(), data); err != nil {
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

// GetAllActors returns a slice of all actors in the StateTree, t.
func GetAllActors(t StateTree) ([]string, []*Actor) {
	st := t.(*stateTree)

	return st.getActorsFromPointers(st.root.Pointers)
}

// GetAllActorsFromStoreFunc is a function with the signature of GetAllActorsFromStore
type GetAllActorsFromStoreFunc = func(context.Context, *hamt.CborIpldStore, *cid.Cid) ([]string, []*Actor, error)

// GetAllActorsFromStore loads a StateTree and returns arrays of addresses and their corresponding actors.
// Third returned value is any error that occurred when loading.
func GetAllActorsFromStore(ctx context.Context, store *hamt.CborIpldStore, stateRoot *cid.Cid) ([]string, []*Actor, error) {
	st, err := LoadStateTree(ctx, store, stateRoot)
	if err != nil {
		return nil, nil, err
	}

	addrs, actors := GetAllActors(st)
	return addrs, actors, nil
}

// NOTE: This extracts actors from pointers recursively. Maybe we shouldn't recurse here.
func (t *stateTree) getActorsFromPointers(ps []*hamt.Pointer) (addresses []string, actors []*Actor) {
	for _, p := range ps {
		for _, kv := range p.KVs {
			a := new(Actor)
			err := a.Unmarshal(kv.Value)
			// An error here means kv.Value was not an actor.
			// We won't append it to our results, but we should keep traversing the tree.
			if err == nil {
				addresses = append(addresses, kv.Key)
				actors = append(actors, a)
			}
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
