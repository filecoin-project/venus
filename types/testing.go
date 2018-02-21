package types

import (
	"context"
	"fmt"
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
			Address(s+"-from"),
			Address(s+"-to"),
			nil,
			s+"-method",
			nil)
	}
}

// FakeStateTree is a fake state tree taht implements types.StateTreeInterface.
type FakeStateTree struct{}

var _ StateTree = FakeStateTree{}

// Flush pretends to Flush the tree.
func (f FakeStateTree) Flush(ctx context.Context) (*cid.Cid, error) {
	return SomeCid(), nil
}

// GetActor explodes in your face. It's not implemented.
func (f FakeStateTree) GetActor(context.Context, Address) (*Actor, error) {
	panic("boom -- not implemented")
}

// SetActor explodes in your face. It's not implemented.
func (f FakeStateTree) SetActor(context.Context, Address, *Actor) error {
	panic("boom -- not implemented")
}
