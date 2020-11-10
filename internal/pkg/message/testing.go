package message

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm"
)

// FakeProvider is a chain and actor provider for testing.
// The provider extends a chain.Builder for providing tipsets and maintains an explicit head CID.
// The provider can provide an actor for a single (head, address) pair.
type FakeProvider struct {
	*chain.Builder
	t *testing.T

	head   block.TipSetKey // Provided by GetHead and expected by others
	actors map[address.Address]*types.Actor
}

// NewFakeProvider creates a new builder and wraps with a provider.
// The builder may be accessed by `provider.Builder`.
func NewFakeProvider(t *testing.T) *FakeProvider {
	builder := chain.NewBuilder(t, address.Address{})
	return &FakeProvider{
		Builder: builder,
		t:       t,
		actors:  make(map[address.Address]*types.Actor)}
}

// GetHead returns the head tipset key.
func (p *FakeProvider) GetHead() block.TipSetKey {
	return p.head
}

// Head fulfills the ChainReaderAPI interface
func (p *FakeProvider) Head() block.TipSetKey {
	return p.GetHead()
}

// GetActorAt returns the actor corresponding to (key, addr) if they match those last set.
func (p *FakeProvider) GetActorAt(ctx context.Context, key block.TipSetKey, addr address.Address) (*types.Actor, error) {
	if !key.Equals(p.head) {
		return nil, errors.Errorf("No such tipset %s, expected %s", key, p.head)
	}
	a, ok := p.actors[addr]
	if !ok {
		return nil, xerrors.Errorf("No such address %s", addr.String())
	}
	return a, nil
}

func (p *FakeProvider) LoadMessages(ctx context.Context, cid cid.Cid) ([]*types.SignedMessage, []*types.UnsignedMessage, error) {
	msg := &types.UnsignedMessage{From: address.TestAddress, CallSeqNum: 1}
	return []*types.SignedMessage{{Message: *msg}}, []*types.UnsignedMessage{msg}, nil
}

// SetHead sets the head tipset
func (p *FakeProvider) SetHead(head block.TipSetKey) {
	_, e := p.GetTipSet(head)
	require.NoError(p.t, e)
	p.head = head
}

// SetActor sets an actor to be mocked on chain
func (p *FakeProvider) SetActor(addr address.Address, act *types.Actor) {
	p.actors[addr] = act
}

// SetHeadAndActor sets the head tipset, along with the from address and actor to be provided.
func (p *FakeProvider) SetHeadAndActor(t *testing.T, head block.TipSetKey, addr address.Address, actor *types.Actor) {
	p.SetHead(head)
	p.SetActor(addr, actor)
}

// MockPublisher is a publisher which just stores the last message published.
type MockPublisher struct {
	ReturnError error                // Error to be returned by Publish()
	Message     *types.SignedMessage // Message received by Publish()
	Height      abi.ChainEpoch       // Height received by Publish()
	Bcast       bool                 // was this broadcast?
}

// Publish records the message etc for subsequent inspection.
func (p *MockPublisher) Publish(ctx context.Context, message *types.SignedMessage, height abi.ChainEpoch, bcast bool) error {
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
func (v FakeValidator) ValidateSignedMessageSyntax(ctx context.Context, msg *types.SignedMessage) error {
	if v.RejectMessages {
		return errors.New("rejected for testing")
	}
	return nil
}

// NullPolicy is a policy that does nothing.
type NullPolicy struct {
}

func (p NullPolicy) MessagesForTipset(ctx context.Context, set *block.TipSet) ([]types.ChainMsg, error) {
	panic("implement me")
}

// HandleNewHead does nothing.
func (NullPolicy) HandleNewHead(ctx context.Context, target PolicyTarget, oldChain, newChain []*block.TipSet) error {
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

type MockGasPredictor struct {
	gas string
}

func NewGasPredictor(gas string) *MockGasPredictor {
	return &MockGasPredictor{
		gas: gas,
	}
}

func (gas *MockGasPredictor) CallWithGas(ctx context.Context, msg *types.UnsignedMessage) (*vm.Ret, error) {
	return &vm.Ret{}, nil
}
