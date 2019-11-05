package message

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// FakeProvider is a chain and actor provider for testing.
// The provider extends a chain.Builder for providing tipsets and maintains an explicit head CID.
// The provider can provide an actor for a single (head, address) pair.
type FakeProvider struct {
	*chain.Builder
	t *testing.T

	head  block.TipSetKey // Provided by GetHead and expected by others
	addr  address.Address // Expected by GetActorAt
	actor *actor.Actor    // Provided by GetActorAt(head, tipKey, addr)
}

// NewFakeProvider creates a new builder and wraps with a provider.
// The builder may be accessed by `provider.Builder`.
func NewFakeProvider(t *testing.T) *FakeProvider {
	builder := chain.NewBuilder(t, address.Address{})
	return &FakeProvider{Builder: builder, t: t}
}

// GetHead returns the head tipset key.
func (p *FakeProvider) GetHead() block.TipSetKey {
	return p.head
}

// GetActorAt returns the actor corresponding to (key, addr) if they match those last set.
func (p *FakeProvider) GetActorAt(ctx context.Context, key block.TipSetKey, addr address.Address) (*actor.Actor, error) {
	if !key.Equals(p.head) {
		return nil, errors.Errorf("No such tipset %s, expected %s", key, p.head)
	}
	if addr != p.addr {
		return nil, errors.Errorf("No such address %s, expected %s", addr, p.addr)
	}
	return p.actor, nil
}

// SetHead sets the head tipset
func (p *FakeProvider) SetHead(head block.TipSetKey) {
	_, e := p.GetTipSet(head)
	require.NoError(p.t, e)
	p.head = head
}

// SetHeadAndActor sets the head tipset, along with the from address and actor to be provided.
func (p *FakeProvider) SetHeadAndActor(t *testing.T, head block.TipSetKey, addr address.Address, actor *actor.Actor) {
	p.SetHead(head)
	p.addr = addr
	p.actor = actor
}

// MockPublisher is a publisher which just stores the last message published.
type MockPublisher struct {
	ReturnError error                // Error to be returned by Publish()
	Message     *types.SignedMessage // Message received by Publish()
	Height      uint64               // Height received by Publish()
	Bcast       bool                 // was this broadcast?
}

// Publish records the message etc for subsequent inspection.
func (p *MockPublisher) Publish(ctx context.Context, message *types.SignedMessage, height uint64, bcast bool) error {
	p.Message = message
	p.Height = height
	p.Bcast = bcast
	return p.ReturnError
}

// FakeValidator is a validator which configurably accepts or rejects messages.
type FakeValidator struct {
	RejectMessages bool
}

// Validate returns an error only if `RejectMessages` is true.
func (v FakeValidator) Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error {
	if v.RejectMessages {
		return errors.New("rejected for testing")
	}
	return nil
}

// NullPolicy is a policy that does nothing.
type NullPolicy struct {
}

// HandleNewHead does nothing.
func (NullPolicy) HandleNewHead(ctx context.Context, target PolicyTarget, oldChain, newChain []block.TipSet) error {
	return nil
}

// MockNetworkPublisher records the last message published.
type MockNetworkPublisher struct {
	Data []byte
}

// Publish records the topic and message.
func (p *MockNetworkPublisher) Publish(ctx context.Context, data []byte) error {
	p.Data = data
	return nil
}
