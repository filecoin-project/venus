package testhelpers

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-bls-sigs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	cid "github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// RequireSignedTestBlockFromTipSet creates a block with a valid signature by
// the passed in miner work and a Miner field set to the minerAddr.
func RequireSignedTestBlockFromTipSet(t *testing.T, baseTipSet block.TipSet, stateRootCid cid.Cid, receiptRootCid cid.Cid, height uint64, minerAddr address.Address, minerWorker address.Address, signer types.Signer) *block.Block {
	electionProof := consensus.MakeFakeElectionProofForTest()
	ticket := consensus.MakeFakeTicketForTest()
	emptyBLSSig := (*bls.Aggregate([]bls.Signature{}))[:]

	b := &block.Block{
		Miner:           minerAddr,
		Ticket:          ticket,
		Parents:         baseTipSet.Key(),
		ParentWeight:    types.Uint64(10000 * height),
		Height:          types.Uint64(height),
		StateRoot:       stateRootCid,
		MessageReceipts: receiptRootCid,
		ElectionProof:   electionProof,
		BLSAggregateSig: emptyBLSSig,
	}
	sig, err := signer.SignBytes(b.SignatureData(), minerWorker)
	require.NoError(t, err)
	b.BlockSig = sig

	return b
}

// MakeRandomPoStProofForTest creates a random proof.
func MakeRandomPoStProofForTest() types.PoStProof {
	proofSize := types.OnePoStProofPartition.ProofLen()
	p := MakeRandomBytes(proofSize)
	p[0] = 42
	poStProof := make([]byte, proofSize)
	for idx, elem := range p {
		poStProof[idx] = elem
	}
	return poStProof
}

// FakeBlockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type FakeBlockRewarder struct{}

var _ consensus.BlockRewarder = (*FakeBlockRewarder)(nil)

// BlockReward is a noop
func (tbr *FakeBlockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	// do nothing to keep state root the same
	return nil
}

// GasReward does nothing
func (tbr *FakeBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.UnsignedMessage, cost types.AttoFIL) error {
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
func (fbv *FakeBlockValidator) ValidateSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error {
	return nil
}

// ValidateSyntax does nothing.
func (fbv *FakeBlockValidator) ValidateSyntax(ctx context.Context, blk *block.Block) error {
	return nil
}

// ValidateMessagesSyntax does nothing
func (fbv *FakeBlockValidator) ValidateMessagesSyntax(ctx context.Context, messages []*types.SignedMessage) error {
	return nil
}

// ValidateUnsignedMessagesSyntax does nothing
func (fbv *FakeBlockValidator) ValidateUnsignedMessagesSyntax(ctx context.Context, messages []*types.UnsignedMessage) error {
	return nil
}

// ValidateReceiptsSyntax does nothing
func (fbv *FakeBlockValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []*types.MessageReceipt) error {
	return nil
}

// StubBlockValidator is a mockable block validator.
type StubBlockValidator struct {
	syntaxStubs   map[cid.Cid]error
	semanticStubs map[cid.Cid]error
}

// NewStubBlockValidator creates a StubBlockValidator that allows errors to configured
// for blocks passed to the Validate* methods.
func NewStubBlockValidator() *StubBlockValidator {
	return &StubBlockValidator{
		syntaxStubs:   make(map[cid.Cid]error),
		semanticStubs: make(map[cid.Cid]error),
	}
}

// ValidateSemantic returns nil or error for stubbed block `child`.
func (mbv *StubBlockValidator) ValidateSemantic(ctx context.Context, child *block.Block, parents *block.TipSet, _ uint64) error {
	return mbv.semanticStubs[child.Cid()]
}

// ValidateSyntax return nil or error for stubbed block `blk`.
func (mbv *StubBlockValidator) ValidateSyntax(ctx context.Context, blk *block.Block) error {
	return mbv.syntaxStubs[blk.Cid()]
}

// StubSyntaxValidationForBlock stubs an error when the ValidateSyntax is called
// on the with the given block.
func (mbv *StubBlockValidator) StubSyntaxValidationForBlock(blk *block.Block, err error) {
	mbv.syntaxStubs[blk.Cid()] = err
}

// StubSemanticValidationForBlock stubs an error when the ValidateSemantic is called
// on the with the given child block.
func (mbv *StubBlockValidator) StubSemanticValidationForBlock(child *block.Block, err error) {
	mbv.semanticStubs[child.Cid()] = err
}

// NewFakeProcessor creates a processor with a test validator and test rewarder
func NewFakeProcessor() *consensus.DefaultProcessor {
	return consensus.NewConfiguredProcessor(&consensus.FakeMessageValidator{}, &FakeBlockRewarder{}, builtin.DefaultActors)
}

type testSigner struct{}

func (ms testSigner) SignBytes(data []byte, addr address.Address) (types.Signature, error) {
	return types.Signature{}, nil
}

// ApplyTestMessage sends a message directly to the vm, bypassing message
// validation
func ApplyTestMessage(st state.Tree, store vm.StorageMap, msg *types.UnsignedMessage, bh *types.BlockHeight) (*consensus.ApplicationResult, error) {
	return applyTestMessageWithAncestors(builtin.DefaultActors, st, store, msg, bh, nil)
}

// ApplyTestMessageWithActors sends a message directly to the vm with a given set of builtin actors
func ApplyTestMessageWithActors(actors builtin.Actors, st state.Tree, store vm.StorageMap, msg *types.UnsignedMessage, bh *types.BlockHeight) (*consensus.ApplicationResult, error) {
	return applyTestMessageWithAncestors(actors, st, store, msg, bh, nil)
}

// ApplyTestMessageWithGas uses the FakeBlockRewarder but the default SignedMessageValidator
func ApplyTestMessageWithGas(actors builtin.Actors, st state.Tree, store vm.StorageMap, msg *types.UnsignedMessage, bh *types.BlockHeight, minerOwner address.Address) (*consensus.ApplicationResult, error) {
	applier := consensus.NewConfiguredProcessor(consensus.NewDefaultMessageValidator(), consensus.NewDefaultBlockRewarder(), actors)
	return newMessageApplier(msg, applier, st, store, bh, minerOwner, nil)
}

func newMessageApplier(msg *types.UnsignedMessage, processor *consensus.DefaultProcessor, st state.Tree, storageMap vm.StorageMap,
	bh *types.BlockHeight, minerOwner address.Address, ancestors []block.TipSet) (*consensus.ApplicationResult, error) {
	amr, err := processor.ApplyMessagesAndPayRewards(context.Background(), st, storageMap, []*types.UnsignedMessage{msg}, minerOwner, bh, ancestors)
	if err != nil {
		return nil, err
	}

	if len(amr) > 0 {
		return &amr[0].ApplicationResult, err
	}

	return nil, err
}

// CreateAndApplyTestMessageFrom wraps the given parameters in a message and calls ApplyTestMessage.
func CreateAndApplyTestMessageFrom(t *testing.T, st state.Tree, vms vm.StorageMap, from address.Address, to address.Address, val, bh uint64, method types.MethodID, ancestors []block.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
	t.Helper()

	pdata := actor.MustConvertParams(params...)
	msg := types.NewUnsignedMessage(from, to, 0, types.NewAttoFILFromFIL(val), method, pdata)
	return applyTestMessageWithAncestors(builtin.DefaultActors, st, vms, msg, types.NewBlockHeight(bh), ancestors)
}

// CreateAndApplyTestMessage wraps the given parameters in a message and calls
// CreateAndApplyTestMessageFrom sending the message from address.TestAddress
func CreateAndApplyTestMessage(t *testing.T, st state.Tree, vms vm.StorageMap, to address.Address, val, bh uint64, method types.MethodID, ancestors []block.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
	return CreateAndApplyTestMessageFrom(t, st, vms, address.TestAddress, to, val, bh, method, ancestors, params...)
}

func applyTestMessageWithAncestors(actors builtin.Actors, st state.Tree, store vm.StorageMap, msg *types.UnsignedMessage, bh *types.BlockHeight, ancestors []block.TipSet) (*consensus.ApplicationResult, error) {
	msg.GasPrice = types.NewGasPrice(1)
	msg.GasLimit = types.NewGasUnits(300)

	ta := newTestApplier(actors)
	return newMessageApplier(msg, ta, st, store, bh, address.Undef, ancestors)
}

func newTestApplier(actors builtin.Actors) *consensus.DefaultProcessor {
	return consensus.NewConfiguredProcessor(&consensus.FakeMessageValidator{}, &FakeBlockRewarder{}, actors)
}
