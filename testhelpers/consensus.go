package testhelpers

import (
	"context"
	"testing"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// TestView is an implementation of stateView used for testing the chain
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

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*types.Block) types.TipSet {
	ts, err := types.NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}

// TestPowerTableView is an implementation of the powertable view used for testing mining
// wherein each miner has totalPower/minerPower power.
type TestPowerTableView struct{ minerPower, totalPower *types.BytesAmount }

// NewTestPowerTableView creates a test power view with the given total power
func NewTestPowerTableView(minerPower *types.BytesAmount, totalPower *types.BytesAmount) *TestPowerTableView {
	return &TestPowerTableView{minerPower: minerPower, totalPower: totalPower}
}

// Total always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	return tv.totalPower, nil
}

// Miner always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return tv.minerPower, nil
}

// HasPower always returns true.
func (tv *TestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

// NewValidTestBlockFromTipSet creates a block for when proofs & power table don't need
// to be correct
func NewValidTestBlockFromTipSet(baseTipSet types.TipSet, stateRootCid cid.Cid, height uint64, minerAddr address.Address, minerWorker address.Address, signer consensus.TicketSigner) *types.Block {
	poStProof := MakeRandomPoStProofForTest()
	ticket, _ := consensus.CreateTicket(poStProof, minerWorker, signer)

	return &types.Block{
		Miner:        minerAddr,
		Ticket:       ticket,
		Parents:      baseTipSet.Key(),
		ParentWeight: types.Uint64(10000 * height),
		Height:       types.Uint64(height),
		Nonce:        types.Uint64(height),
		StateRoot:    stateRootCid,
		Proof:        poStProof,
	}
}

// MakeRandomPoStProofForTest creates a random proof.
func MakeRandomPoStProofForTest() []byte {
	proofSize := types.OnePoStProofPartition.ProofLen()
	p := MakeRandomBytes(proofSize)
	p[0] = 42
	poStProof := make([]byte, proofSize)
	for idx, elem := range p {
		poStProof[idx] = elem
	}
	return poStProof
}

// TestSignedMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type TestSignedMessageValidator struct{}

var _ consensus.SignedMessageValidator = (*TestSignedMessageValidator)(nil)

// Validate always returns nil
func (tsmv *TestSignedMessageValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error {
	return nil
}

// TestBlockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type TestBlockRewarder struct{}

var _ consensus.BlockRewarder = (*TestBlockRewarder)(nil)

// BlockReward is a noop
func (tbr *TestBlockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	// do nothing to keep state root the same
	return nil
}

// GasReward does nothing
func (tbr *TestBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, cost types.AttoFIL) error {
	// do nothing to keep state root the same
	return nil
}

// FakeBlockValidator passes everything as valid
type FakeBlockValidator struct{}

// NewFakeBlockValidator createas a FakeBlockValidator that passes everything as valid.
func NewFakeBlockValidator() *FakeBlockValidator {
	return &FakeBlockValidator{}
}

// ValidateSemantic does nothing.
func (fbv *FakeBlockValidator) ValidateSemantic(ctx context.Context, child *types.Block, parents *types.TipSet) error {
	return nil
}

// ValidateSyntax does nothing.
func (fbv *FakeBlockValidator) ValidateSyntax(ctx context.Context, blk *types.Block) error {
	return nil
}

// NewTestProcessor creates a processor with a test validator and test rewarder
func NewTestProcessor() *consensus.DefaultProcessor {
	return consensus.NewConfiguredProcessor(&TestSignedMessageValidator{}, &TestBlockRewarder{})
}

type testSigner struct{}

func (ms testSigner) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return types.Signature{}, nil
}

// ApplyTestMessage sends a message directly to the vm, bypassing message
// validation
func ApplyTestMessage(st state.Tree, store vm.StorageMap, msg *types.Message, bh *types.BlockHeight) (*consensus.ApplicationResult, error) {
	return applyTestMessageWithAncestors(st, store, msg, bh, nil)
}

// ApplyTestMessageWithGas uses the TestBlockRewarder but the default SignedMessageValidator
func ApplyTestMessageWithGas(st state.Tree, store vm.StorageMap, msg *types.Message, bh *types.BlockHeight, signer *types.MockSigner,
	gasPrice types.AttoFIL, gasLimit types.GasUnits, minerOwner address.Address) (*consensus.ApplicationResult, error) {

	smsg, err := types.NewSignedMessage(*msg, signer, gasPrice, gasLimit)
	if err != nil {
		panic(err)
	}
	applier := consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), consensus.NewDefaultBlockRewarder())
	return newMessageApplier(smsg, applier, st, store, bh, minerOwner, nil)
}

func newMessageApplier(smsg *types.SignedMessage, processor *consensus.DefaultProcessor, st state.Tree, storageMap vm.StorageMap,
	bh *types.BlockHeight, minerOwner address.Address, ancestors []types.TipSet) (*consensus.ApplicationResult, error) {
	amr, err := processor.ApplyMessagesAndPayRewards(context.Background(), st, storageMap, []*types.SignedMessage{smsg}, minerOwner, bh, ancestors)

	if len(amr.Results) > 0 {
		return amr.Results[0], err
	}

	return nil, err
}

// CreateAndApplyTestMessageFrom wraps the given parameters in a message and calls ApplyTestMessage.
func CreateAndApplyTestMessageFrom(t *testing.T, st state.Tree, vms vm.StorageMap, from address.Address, to address.Address, val, bh uint64, method string, ancestors []types.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
	t.Helper()

	pdata := actor.MustConvertParams(params...)
	msg := types.NewMessage(from, to, 0, types.NewAttoFILFromFIL(val), method, pdata)
	return applyTestMessageWithAncestors(st, vms, msg, types.NewBlockHeight(bh), ancestors)
}

// CreateAndApplyTestMessage wraps the given parameters in a message and calls
// CreateAndApplyTestMessageFrom sending the message from address.TestAddress
func CreateAndApplyTestMessage(t *testing.T, st state.Tree, vms vm.StorageMap, to address.Address, val, bh uint64, method string, ancestors []types.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
	return CreateAndApplyTestMessageFrom(t, st, vms, address.TestAddress, to, val, bh, method, ancestors, params...)
}

func applyTestMessageWithAncestors(st state.Tree, store vm.StorageMap, msg *types.Message, bh *types.BlockHeight, ancestors []types.TipSet) (*consensus.ApplicationResult, error) {
	smsg, err := types.NewSignedMessage(*msg, testSigner{}, types.NewGasPrice(1), types.NewGasUnits(300))
	if err != nil {
		panic(err)
	}

	ta := newTestApplier()
	return newMessageApplier(smsg, ta, st, store, bh, address.Undef, ancestors)
}

func newTestApplier() *consensus.DefaultProcessor {
	return consensus.NewConfiguredProcessor(&TestSignedMessageValidator{}, &TestBlockRewarder{})
}
