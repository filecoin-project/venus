package testhelpers

import (
	"context"
	"errors"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm"
	"github.com/filecoin-project/venus/internal/pkg/vm/state"
)

// RequireMakeStateTree takes a map of addresses to actors and stores them on
// the state tree, requiring that all its steps succeed.
func RequireMakeStateTree(t *testing.T, cst cbor.IpldStore, acts map[address.Address]*types.Actor) (cid.Cid, *state.State) {
	ctx := context.Background()
	tree, err := state.NewState(cst, state.StateTreeVersion0)
	if err != nil {
		t.Fatal(err)
	}

	for addr, act := range acts {
		err := tree.SetActor(ctx, addr, act)
		require.NoError(t, err)
	}

	c, err := tree.Flush(ctx)
	require.NoError(t, err)

	return c, tree
}

// RequireNewMinerActor creates a new miner actor with the given owner, pledge, and collateral,
// and requires that its steps succeed.
func RequireNewMinerActor(ctx context.Context, t *testing.T, st state.State, vms vm.Storage, owner address.Address, pledge uint64, pid peer.ID, coll types.AttoFIL) (*types.Actor, address.Address) {
	// Dragons: re-write using the new actor states structures directly

	return nil, address.Undef
}

// RequireLookupActor converts the given address to an id address before looking up the actor in the state tree
func RequireLookupActor(ctx context.Context, t *testing.T, st state.State, vms vm.Storage, actorAddr address.Address) (*types.Actor, address.Address) {
	// Dragons: delete, nothing outside the vm should be concerned about actor id indexes

	return nil, address.Undef
}

// RequireNewFakeActor instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActor(t *testing.T, vms vm.Storage, addr address.Address, codeCid cid.Cid) *types.Actor {
	return RequireNewFakeActorWithTokens(t, vms, addr, codeCid, types.NewAttoFILFromFIL(100))
}

// RequireNewFakeActorWithTokens instantiates and returns a new fake actor and requires
// that its steps succeed.
func RequireNewFakeActorWithTokens(t *testing.T, vms vm.Storage, addr address.Address, codeCid cid.Cid, amt types.AttoFIL) *types.Actor {

	return nil
}

// RequireNewInitActor instantiates and returns a new init actor
func RequireNewInitActor(t *testing.T, vms vm.Storage) *types.Actor {

	return nil
}

// RequireRandomPeerID returns a new libp2p peer ID or panics.
func RequireRandomPeerID(t *testing.T) peer.ID {
	pid, err := RandPeerID()
	require.NoError(t, err)
	return pid
}

// MockMessagePoolValidator is a mock validator
type MockMessagePoolValidator struct {
	Valid bool
}

// NewMockMessagePoolValidator creates a MockMessagePoolValidator
func NewMockMessagePoolValidator() *MockMessagePoolValidator {
	return &MockMessagePoolValidator{Valid: true}
}

// Validate returns true if the mock validator is set to validate the message
func (v *MockMessagePoolValidator) ValidateSignedMessageSyntax(ctx context.Context, msg *types.SignedMessage) error {
	if v.Valid {
		return nil
	}
	return errors.New("mock validation error")
}
