package state

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// Review: can we get rid of this?
type Tree interface {
	Root() (Root, bool)

	SetActor(ctx context.Context, key actorKey, a *actor.Actor) error
	GetActor(ctx context.Context, key actorKey) (*actor.Actor, bool, error)
	DeleteActor(ctx context.Context, key actorKey) error

	Rollback(ctx context.Context, root Root) error
	Commit(ctx context.Context) (Root, error)

	GetAllActors(ctx context.Context) <-chan GetAllActorsResult
}

// TreeBitWidth is the bit width of the HAMT used to store a state tree
const TreeBitWidth = 5

// Root is the root type of a state.
//
// Any and all state can be identified by this.
//
// Note: it might not be possible to locally reconstruct the entire state if the some parts are missing.
type Root = cid.Cid

type actorKey = address.Address

// State is the VM state manager.
type State struct {
	store    cbor.IpldStore
	dirty    bool
	root     Root       // The last committed root.
	rootNode *hamt.Node // The current (not necessarily committed) root node.
}

// NewState creates a new VM state.
func NewState(store cbor.IpldStore) *State {
	st := newState(store, cid.Undef, hamt.NewNode(store, hamt.UseTreeBitWidth(TreeBitWidth)))
	st.dirty = true
	return st
}

// LoadState creates a new VMStorage.
func LoadState(ctx context.Context, store cbor.IpldStore, root Root) (*State, error) {
	rootNode, err := hamt.LoadNode(ctx, store, root, hamt.UseTreeBitWidth(TreeBitWidth))

	if err != nil {
		return nil, errors.Wrapf(err, "failed to load node for %s", root)
	}

	return newState(store, root, rootNode), nil
}

func newState(store cbor.IpldStore, root Root, rootNode *hamt.Node) *State {
	return &State{
		store:    store,
		dirty:    false,
		root:     root,
		rootNode: rootNode,
	}
}

// GetActor retrieves an actor by their key.
// If the actor is not found it will return false and no error.
// If there are any IO or decoding errors, it will return false and the error.
func (st *State) GetActor(ctx context.Context, key actorKey) (*actor.Actor, bool, error) {
	actorBytes, err := st.rootNode.FindRaw(ctx, string(key.Bytes()))
	if err == hamt.ErrNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var act actor.Actor
	err = encoding.Decode(actorBytes, &act)
	if err != nil {
		return nil, false, err
	}

	return &act, true, nil
}

// SetActor sets the the actor to the given value whether it previously existed or not.
//
// This method will not check if the actor previuously existed, it will blindly overwrite it.
func (st *State) SetActor(ctx context.Context, key actorKey, a *actor.Actor) error {
	actBytes, err := encoding.Encode(a)
	if err != nil {
		return err
	}
	if err := st.rootNode.SetRaw(ctx, string(key.Bytes()), actBytes); err != nil {
		return errors.Wrap(err, "setting actor in state tree failed")
	}
	st.dirty = true
	return nil
}

// DeleteActor remove the actor from the storage.
//
// This method will NOT return an error if the actor was not found.
// This behaviour is based on a principle that some store implementations might not be able to determine
// whether something exists before deleting it.
func (st *State) DeleteActor(ctx context.Context, key actorKey) error {
	err := st.rootNode.Delete(ctx, string(key.Bytes()))
	st.dirty = true
	if err == hamt.ErrNotFound {
		return nil
	}
	return err
}

// Commit will flush the state tree into the backing store.
// The new root is returned.
func (st *State) Commit(ctx context.Context) (Root, error) {
	if err := st.rootNode.Flush(ctx); err != nil {
		return cid.Undef, err
	}

	root, err := st.store.Put(ctx, st.rootNode)
	if err != nil {
		return cid.Undef, err
	}

	st.root = root
	st.dirty = false
	return root, nil
}

// Rollback resets the root to a provided value.
func (st *State) Rollback(ctx context.Context, root Root) error {
	// load the original root node again
	rootNode, err := hamt.LoadNode(ctx, st.store, root, hamt.UseTreeBitWidth(TreeBitWidth))
	if err != nil {
		return errors.Wrapf(err, "failed to load node for %s", root)
	}

	// reset the root node
	st.rootNode = rootNode
	st.dirty = false
	return nil
}

// Root returns the last committed root of the tree and whether any writes have since occurred.
func (st *State) Root() (Root, bool) {
	return st.root, st.dirty
}

// GetAllActorsResult is the struct returned via a channel by the GetAllActors
// method. This struct contains only an address string and the actor itself.
type GetAllActorsResult struct {
	Key   actorKey
	Actor *actor.Actor
	Error error
}

// GetAllActors returns a channel which provides all actors in the StateTree.
func (st *State) GetAllActors(ctx context.Context) <-chan GetAllActorsResult {
	out := make(chan GetAllActorsResult)
	go func() {
		defer close(out)
		st.getActorsFromPointers(ctx, out, st.rootNode.Pointers)
	}()
	return out
}

// NOTE: This extracts actors from pointers recursively. Maybe we shouldn't recurse here.
func (st *State) getActorsFromPointers(ctx context.Context, out chan<- GetAllActorsResult, ps []*hamt.Pointer) {
	for _, p := range ps {
		for _, kv := range p.KVs {
			var a actor.Actor

			if err := encoding.Decode(kv.Value.Raw, &a); err != nil {
				fmt.Printf("bad raw bytes: %x\n", kv.Value.Raw)
				panic(err)
			}

			select {
			case <-ctx.Done():
				out <- GetAllActorsResult{
					Error: ctx.Err(),
				}
				return
			default:
				addr, err := address.NewFromBytes(kv.Key)
				if err != nil {
					fmt.Printf("bad address key bytes: %x\n", kv.Value.Raw)
					panic(err)
				}
				out <- GetAllActorsResult{
					Key:   addr,
					Actor: &a,
				}
			}
		}
		if p.Link.Defined() {
			n, err := hamt.LoadNode(ctx, st.store, p.Link, hamt.UseTreeBitWidth(TreeBitWidth))
			// Even if we hit an error and can't follow this link, we should
			// keep traversing its siblings.
			if err != nil {
				continue
			}
			st.getActorsFromPointers(ctx, out, n.Pointers)
		}
	}
}
