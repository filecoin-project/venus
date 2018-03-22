package types

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"

	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
)

// Type-related test helpers.

// SomeCid generates a Cid for use in tests where you want a Cid but don't care
// what it is.
func SomeCid() *cid.Cid {
	b := &Block{}
	return b.Cid()
}

// NewCidForTestGetter returns a closure that returns a Cid unique to that invocation.
// The Cid is unique wrt the closure returned, not globally. You can use this function
// in tests.
func NewCidForTestGetter() func() *cid.Cid {
	i := uint64(31337)
	return func() *cid.Cid {
		b := &Block{Height: i}
		i++
		return b.Cid()
	}
}

// NewAddressForTestGetter returns a closure that returns an address unique to that invocation.
// The address is unique wrt the closure returned, not globally.
func NewAddressForTestGetter() func() Address {
	i := 0
	return func() Address {
		s := fmt.Sprintf("address%d", i)
		i++
		return MakeTestAddress(s)
	}
}

// NewMessageForTestGetter returns a closure that returns a message unique to that invocation.
// The message is unique wrt the closure returned, not globally. You can use this function
// in tests instead of manually creating messages -- it both reduces duplication and gives us
// exactly one place to create valid messages for tests if messages require validation in the
// future.
func NewMessageForTestGetter() func() *Message {
	i := 0
	return func() *Message {
		s := fmt.Sprintf("msg%d", i)
		i++
		return NewMessage(
			NewMainnetAddress([]byte(s+"-from")),
			NewMainnetAddress([]byte(s+"-to")),
			nil,
			s,
			nil)
	}
}

// NewMsgs returns n messages. The messages returned are unique to this invocation
// but are not unique globally (ie, a second call to NewMsgs will return the same
// set of messages).
func NewMsgs(n int) []*Message {
	newMsg := NewMessageForTestGetter()
	msgs := make([]*Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = newMsg()
	}
	return msgs
}

// MsgCidsEqual returns true if the message cids are equal. It panics if
// it can't get their cid.
func MsgCidsEqual(m1, m2 *Message) bool {
	m1Cid, err := m1.Cid()
	if err != nil {
		panic(err)
	}
	m2Cid, err := m2.Cid()
	if err != nil {
		panic(err)
	}
	return m1Cid.Equals(m2Cid)
}

// MockStateTree is a testify mock that implements StateTree.
type MockStateTree struct {
	mock.Mock
}

var _ StateTree = &MockStateTree{}

// Flush implements StateTree.Flush.
func (m *MockStateTree) Flush(ctx context.Context) (c *cid.Cid, err error) {
	args := m.Called(ctx)
	if args.Get(0) != nil {
		c = args.Get(0).(*cid.Cid)
	}
	err = args.Error(1)
	return
}

// GetActor implements StateTree.GetActor.
func (m *MockStateTree) GetActor(ctx context.Context, address Address) (actor *Actor, err error) {
	args := m.Called(ctx, address)
	if args.Get(0) != nil {
		actor = args.Get(0).(*Actor)
	}
	err = args.Error(1)
	return
}

// SetActor implements StateTree.SetActor.
func (m *MockStateTree) SetActor(ctx context.Context, address Address, actor *Actor) error {
	args := m.Called(ctx, address, actor)
	return args.Error(0)
}

// GetOrCreateActor implements StateTree.GetOrCreateActor.
func (m *MockStateTree) GetOrCreateActor(ctx context.Context, address Address, creator func() (*Actor, error)) (*Actor, error) {
	panic("do not call me")
}

// Snapshot implements StateTree.Snapshot.
func (m *MockStateTree) Snapshot() RevID {
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
