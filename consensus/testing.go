package consensus

import (
	"context"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/proofs"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"

	"github.com/stretchr/testify/require"
)

// TestView is an implementation of stateView used for testing the chain
// manager.  It provides a consistent view that the storage market
// stores 1 byte and all miners store 0 bytes regardless of inputs.
type TestView struct{}

var _ PowerTableView = &TestView{}

// Total always returns 1.
func (tv *TestView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error) {
	return uint64(1), nil
}

// Miner always returns 1.
func (tv *TestView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (uint64, error) {
	return uint64(1), nil
}

// HasPower always returns true.
func (tv *TestView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*types.Block) TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// RequireTipSetAdd adds a block to the provided tipset and requires that this
// does not error.
func RequireTipSetAdd(require *require.Assertions, blk *types.Block, ts TipSet) {
	err := ts.AddBlock(blk)
	require.NoError(err)
}

// TestPowerTableView is an implementation of the powertable view used for testing mining
// wherein each miner has totalPower/minerPower power.
type TestPowerTableView struct{ minerPower, totalPower uint64 }

// NewTestPowerTableView creates a test power view with the given total power
func NewTestPowerTableView(minerPower uint64, totalPower uint64) *TestPowerTableView {
	return &TestPowerTableView{minerPower: minerPower, totalPower: totalPower}
}

// Total always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (uint64, error) {
	return tv.totalPower, nil
}

// Miner always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (uint64, error) {
	return tv.minerPower, nil
}

// HasPower always returns true.
func (tv *TestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

// NewValidTestBlockFromTipSet creates a block for when proofs & power table don't need
// to be correct
func NewValidTestBlockFromTipSet(baseTipSet TipSet, height uint64, minerAddr address.Address) *types.Block {
	postProof := MakeRandomPoSTProofForTest()
	ticket := CreateTicket(postProof, minerAddr)

	baseTsBlock := baseTipSet.ToSlice()[0]
	stateRoot := baseTsBlock.StateRoot

	return &types.Block{
		Miner:        minerAddr,
		Ticket:       ticket,
		Parents:      baseTipSet.ToSortedCidSet(),
		ParentWeight: types.Uint64(10000 * height),
		Height:       types.Uint64(height),
		Nonce:        types.Uint64(height),
		StateRoot:    stateRoot,
		Proof:        postProof,
	}
}

// MakeRandomPoSTProofForTest creates a random proof.
func MakeRandomPoSTProofForTest() proofs.PoStProof {
	p := testhelpers.MakeRandomBytes(192)
	p[0] = 42
	var postProof proofs.PoStProof
	for idx, elem := range p {
		postProof[idx] = elem
	}
	return postProof
}
