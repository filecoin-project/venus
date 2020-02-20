package state

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// CachedTree is a read-through cache on top of a state tree.
type CachedTree struct {
	st    Tree
	cache map[address.Address]*actor.Actor
}

// NewCachedTree returns a `CachedTree` based on an existiing `Tree`.
//
// The cache will be empty on construction.
func NewCachedTree(st Tree) *CachedTree {
	return &CachedTree{
		st:    st,
		cache: make(map[address.Address]*actor.Actor),
	}
}

// Flush flushes the commited changes.
func (t *CachedTree) Flush(ctx context.Context) (cid.Cid, error) {
	return t.st.Flush(ctx)
}

// GetActor retrieves an actor from the cache. If it's not found it will get it from the
// underlying tree and then set it in the cache before returning it.
func (t *CachedTree) GetActor(ctx context.Context, a address.Address) (*actor.Actor, error) {
	var err error
	actor, found := t.cache[a]
	if !found {
		actor, err = t.st.GetActor(ctx, a)
		if err != nil {
			return nil, err
		}
		t.cache[a] = actor
	}
	return actor, nil
}

// GetOrCreateActor retrieves an actor from the cache. If it's not found it will GetOrCreate it from the
// underlying tree and then set it in the cache before returning it.
func (t *CachedTree) GetOrCreateActor(ctx context.Context,
	addr address.Address,
	creator func() (*actor.Actor, address.Address, error)) (*actor.Actor, address.Address, error) {

	var err error
	actor, found := t.cache[addr]
	if found {
		return actor, addr, nil
	}

	actor, mappedAddr, err := t.st.GetOrCreateActor(ctx, addr, creator)
	if err != nil {
		return nil, address.Undef, err
	}
	t.cache[mappedAddr] = actor
	return actor, mappedAddr, nil
}

// DeleteActor remove the actor from the storage.
// This method will NOT return an error if the actor was not found.
func (t *CachedTree) DeleteActor(ctx context.Context, addr address.Address) error {
	// delete from cache
	_, found := t.cache[addr]
	if found {
		delete(t.cache, addr)
	}

	// delete from store
	return t.st.DeleteActor(ctx, addr)
}

// Commit takes all the cached actors and sets them into the underlying cache.
func (t *CachedTree) Commit(ctx context.Context) error {
	for addr, actor := range t.cache {
		err := t.st.SetActor(ctx, addr, actor)
		if err != nil {
			return fmt.Errorf("Could not commit cached actors to state tree. %s", err)
		}
	}
	t.cache = make(map[address.Address]*actor.Actor)
	return nil
}
