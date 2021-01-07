package testhelpers

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-state-types/abi"
	fbig "github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/block"
	"math/rand"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/state"
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

// RequireTipset is a helper that constructs a tipset
func RequireTipset(t *testing.T) *block.TipSet {
	return RequireTipsetWithHeight(t, abi.ChainEpoch(rand.Int()))
}

func RequireTipsetWithHeight(t *testing.T, height abi.ChainEpoch) *block.TipSet {
	newAddress := types.NewForTestGetter()
	blk := &block.Block{
		Miner:         newAddress(),
		Ticket:        block.Ticket{VRFProof: []byte{0x03, 0x01, 0x02}},
		ElectionProof: &block.ElectionProof{VRFProof: []byte{0x0c, 0x0d}},
		BeaconEntries: []*block.BeaconEntry{
			{
				Round: 44,
				Data:  []byte{0xc0},
			},
		},
		Height:                height,
		Messages:              types.CidFromString(t, "someothercid"),
		ParentMessageReceipts: types.CidFromString(t, "someothercid"),
		Parents:               block.NewTipSetKey(types.CidFromString(t, "someothercid")),
		ParentWeight:          fbig.NewInt(1),
		ForkSignaling:         2,
		ParentStateRoot:       types.CidFromString(t, "someothercid"),
		Timestamp:             4,
		ParentBaseFee:         abi.NewTokenAmount(20),
		BlockSig: &acrypto.Signature{
			Type: acrypto.SigTypeBLS,
			Data: []byte{0x4},
		},
	}
	b, _ := block.NewTipSet(blk)
	return b
}
