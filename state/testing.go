package state

import (
	"context"
	"fmt"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/stretchr/testify/mock"

	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
)

// MustFlush flushes the StateTree or panics if it can't.
func MustFlush(st Tree) *cid.Cid {
	cid, err := st.Flush(context.Background())
	if err != nil {
		panic(err)
	}
	return cid
}

// MustGetActor gets the actor or panics if it can't.
func MustGetActor(st Tree, a types.Address) *types.Actor {
	actor, err := st.GetActor(context.Background(), a)
	if err != nil {
		panic(err)
	}
	return actor
}

// MustSetActor sets the actor or panics if it can't.
func MustSetActor(st Tree, address types.Address, actor *types.Actor) *cid.Cid {
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
	BuiltinActors map[string]exec.ExecutableActor
}

var _ Tree = &MockStateTree{}

// Flush implements StateTree.Flush.
func (m *MockStateTree) Flush(ctx context.Context) (c *cid.Cid, err error) {
	if m.NoMocks {
		return
	}
	args := m.Called(ctx)
	if args.Get(0) != nil {
		c = args.Get(0).(*cid.Cid)
	}
	err = args.Error(1)
	return
}

// GetActor implements StateTree.GetActor.
func (m *MockStateTree) GetActor(ctx context.Context, address types.Address) (actor *types.Actor, err error) {
	if m.NoMocks {
		return
	}

	args := m.Called(ctx, address)
	if args.Get(0) != nil {
		actor = args.Get(0).(*types.Actor)
	}
	err = args.Error(1)
	return
}

// SetActor implements StateTree.SetActor.
func (m *MockStateTree) SetActor(ctx context.Context, address types.Address, actor *types.Actor) error {
	if m.NoMocks {
		return nil
	}

	args := m.Called(ctx, address, actor)
	return args.Error(0)
}

// GetOrCreateActor implements StateTree.GetOrCreateActor.
func (m *MockStateTree) GetOrCreateActor(ctx context.Context, address types.Address, creator func() (*types.Actor, error)) (*types.Actor, error) {
	panic("do not call me")
}

// Snapshot implements StateTree.Snapshot.
func (m *MockStateTree) Snapshot(ctx context.Context) (RevID, error) {
	panic("do not call me")
}

// RevertTo implements StateTree.RevertTo.
func (m *MockStateTree) RevertTo(RevID) {
	panic("do not call me")
}

// Debug implements StateTree.Debug
func (m *MockStateTree) Debug() {
	panic("do not call me")
}

// GetBuiltinActorCode implements StateTree.GetBuiltinActorCode
func (m *MockStateTree) GetBuiltinActorCode(c *cid.Cid) (exec.ExecutableActor, error) {
	a, ok := m.BuiltinActors[c.KeyString()]
	if !ok {
		return nil, fmt.Errorf("unknown actor: %s", c)
	}

	return a, nil
}
