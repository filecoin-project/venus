package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MustFlush flushes the StateTree or panics if it can't.
func MustFlush(st Tree) cid.Cid {
	cid, err := st.Flush(context.Background())
	if err != nil {
		panic(err)
	}
	return cid
}

// MustGetActor gets the actor or panics if it can't.
func MustGetActor(st Tree, a address.Address) *actor.Actor {
	actor, err := st.GetActor(context.Background(), a)
	if err != nil {
		panic(err)
	}
	return actor
}

// MustSetActor sets the actor or panics if it can't.
func MustSetActor(st Tree, address address.Address, actor *actor.Actor) cid.Cid {
	err := st.SetActor(context.Background(), address, actor)
	if err != nil {
		panic(err)
	}
	return MustFlush(st)
}

// MockStateTree is a testify mock that implements StateTree.
type MockStateTree struct {
	mock.Mock

	NoMocks       bool
	BuiltinActors map[cid.Cid]dispatch.ExecutableActor
}

// GetActorStorage implements Tree interface
func (m *MockStateTree) GetActorStorage(ctx context.Context, a address.Address, stg interface{}) error {
	panic("do not call me")
}

var _ Tree = &MockStateTree{}

// Flush implements StateTree.Flush.
func (m *MockStateTree) Flush(ctx context.Context) (c cid.Cid, err error) {
	if m.NoMocks {
		return
	}
	args := m.Called(ctx)
	if args.Get(0) != nil {
		c = args.Get(0).(cid.Cid)
	}
	err = args.Error(1)
	return
}

// GetActor implements StateTree.GetActorCode.
func (m *MockStateTree) GetActor(ctx context.Context, address address.Address) (a *actor.Actor, err error) {
	if m.NoMocks {
		return
	}

	args := m.Called(ctx, address)
	if args.Get(0) != nil {
		a = args.Get(0).(*actor.Actor)
	}
	err = args.Error(1)
	return
}

// SetActor implements StateTree.SetActor.
func (m *MockStateTree) SetActor(ctx context.Context, address address.Address, actor *actor.Actor) error {
	if m.NoMocks {
		return nil
	}

	args := m.Called(ctx, address, actor)
	return args.Error(0)
}

// GetOrCreateActor implements StateTree.GetOrCreateActor.
func (m *MockStateTree) GetOrCreateActor(ctx context.Context, addr address.Address, creator func() (*actor.Actor, address.Address, error)) (*actor.Actor, address.Address, error) {
	return creator()
}

// ForEachActor implements StateTree.ForEachActor
func (m *MockStateTree) ForEachActor(ctx context.Context, walkFn ActorWalkFn) error {
	panic("Do not call me")
}

// GetAllActors implements StateTree.GetAllActors
func (m *MockStateTree) GetAllActors(ctx context.Context) <-chan GetAllActorsResult {
	panic("do not call me")
}

// GetActorCode implements StateTree.GetActorCode
func (m *MockStateTree) GetActorCode(c cid.Cid, protocol uint64) (dispatch.ExecutableActor, error) {
	a, ok := m.BuiltinActors[c]
	if !ok {
		return nil, fmt.Errorf("unknown actor: %v", c.String())
	}

	return a, nil
}

// TreeFromString sets a state tree based on an int.  TODO: this indirection
// can be avoided when we are able to change cborStore to an interface and then
// making a test implementation of the cbor store that can map test cids to test
// states.
func TreeFromString(t *testing.T, s string, cst hamt.CborIpldStore) Tree {
	tree := NewTree(cst)
	strAddr, err := address.NewSecp256k1Address([]byte(s))
	require.NoError(t, err)
	err = tree.SetActor(context.Background(), strAddr, &actor.Actor{})
	require.NoError(t, err)
	return tree
}
