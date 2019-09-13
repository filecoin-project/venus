package testhelpers

import (
	"context"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// TestView is an implementation of powertableview used for testing the chain
// manager.  It provides a consistent view that the storage market
// stores 1 byte and all miners store 0 bytes regardless of inputs.
type TestView struct{}

var _ consensus.PowerTableView = &TestView{}

// Total always returns 1.
func (tv *TestView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	return types.NewBytesAmount(1), nil
}

// Miner always returns 1.
func (tv *TestView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return types.NewBytesAmount(1), nil
}

// HasPower always returns true.
func (tv *TestView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

// WorkerAddr just returns the miner address.
func (tv *TestView) WorkerAddr(_ context.Context, _ state.Tree, _ blockstore.Blockstore, mAddr address.Address) (address.Address, error) {
	return mAddr, nil
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*types.Block) types.TipSet {
	ts, err := types.NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}

// NewValidTestBlockFromTipSet creates a block for when proofs & power table don't need
// to be correct
func NewValidTestBlockFromTipSet(baseTipSet types.TipSet, stateRootCid cid.Cid, height uint64, minerAddr address.Address, minerWorker address.Address, signer types.Signer) (*types.Block, error) {
	electionProof := consensus.MakeFakeElectionProofForTest()
	ticket := consensus.MakeFakeTicketForTest()

	b := &types.Block{
		Miner:         minerAddr,
		Tickets:       []types.Ticket{ticket},
		Parents:       baseTipSet.Key(),
		ParentWeight:  types.Uint64(10000 * height),
		Height:        types.Uint64(height),
		StateRoot:     stateRootCid,
		ElectionProof: electionProof,
	}
	sig, err := signer.SignBytes(b.SignatureData(), minerWorker)
	if err != nil {
		return nil, err
	}
	b.BlockSig = sig

	return b, nil
}
