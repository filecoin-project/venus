package state

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// CachedTree is a read-through cache on top of a state tree.
type CachedTree struct {
	st    Tree
	cache map[address.Address]*actor.Actor
}

// NewCachedStateTree returns a initialized empty CachedTree
func NewCachedStateTree(st Tree) *CachedTree {
	return &CachedTree{
		st:    st,
		cache: make(map[address.Address]*actor.Actor),
	}
}

// GetBuiltinActorCode simply delegates to the underlying tree
func (t *CachedTree) GetBuiltinActorCode(codePointer cid.Cid) (exec.ExecutableActor, error) {
	return t.st.GetBuiltinActorCode(codePointer)
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
func (t *CachedTree) GetOrCreateActor(ctx context.Context, address address.Address, creator func() (*actor.Actor, error)) (*actor.Actor, error) {
	var err error
	actor, found := t.cache[address]
	if !found {
		actor, err = t.st.GetOrCreateActor(ctx, address, creator)
		if err != nil {
			return nil, err
		}
		t.cache[address] = actor
	}
	return actor, nil
}

// Commit takes all the cached actors and sets them into the underlying cache.
func (t *CachedTree) Commit(ctx context.Context) error {
	for addr, actor := range t.cache {
		err := t.st.SetActor(ctx, addr, actor)
		if err != nil {
			return errors.FaultErrorWrap(err, "Could not commit cached actors to state tree.")
		}
	}
	t.cache = make(map[address.Address]*actor.Actor)
	return nil
}
