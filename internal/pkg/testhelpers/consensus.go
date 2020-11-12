package testhelpers

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// RequireSignedTestBlockFromTipSet creates a block with a valid signature by
// the passed in miner work and a Miner field set to the minerAddr.
func RequireSignedTestBlockFromTipSet(t *testing.T, baseTipSet block.TipSet, stateRootCid cid.Cid, receiptRootCid cid.Cid, height abi.ChainEpoch, minerAddr address.Address, minerWorker address.Address, signer types.Signer) *block.Block {
	ticket := consensus.MakeFakeTicketForTest()
	emptyBLSSig := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: (*bls.Aggregate([]bls.Signature{}))[:],
	}

	b := &block.Block{
		Miner:           minerAddr,
		Ticket:          ticket,
		Parents:         baseTipSet.Key(),
		ParentWeight:    types.Uint64ToBig(uint64(height * 10000)),
		Height:          height,
		StateRoot:       enccid.NewCid(stateRootCid),
		MessageReceipts: enccid.NewCid(receiptRootCid),
		BLSAggregateSig: &emptyBLSSig,
	}
	sig, err := signer.SignBytes(context.TODO(), b.SignatureData(), minerWorker)
	require.NoError(t, err)
	b.BlockSig = &sig

	return b
}

// FakeBlockValidator passes everything as valid
type FakeBlockValidator struct{}

// NewFakeBlockValidator createas a FakeBlockValidator that passes everything as valid.
func NewFakeBlockValidator() *FakeBlockValidator {
	return &FakeBlockValidator{}
}

// ValidateHeaderSemantic does nothing.
func (fbv *FakeBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *block.Block, parents block.TipSet) error {
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
func (fbv *FakeBlockValidator) ValidateReceiptsSyntax(ctx context.Context, receipts []types.MessageReceipt) error {
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

// ValidateHeaderSemantic returns nil or error for stubbed block `child`.
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

// StubSemanticValidationForBlock stubs an error when the ValidateHeaderSemantic is called
// on the with the given child block.
func (mbv *StubBlockValidator) StubSemanticValidationForBlock(child *block.Block, err error) {
	mbv.semanticStubs[child.Cid()] = err
}

//
//// NewFakeProcessor creates a processor with a test validator and test rewarder
//func NewFakeProcessor() *consensus.DefaultProcessor {
//	return consensus.NewConfiguredProcessor(vm.DefaultActors, &vm.FakeSyscalls{}, &consensus.FakeChainRandomness{})
//}
//
//// ApplyTestMessage sends a message directly to the vm, bypassing message
//// validation
//func ApplyTestMessage(st state.State, store vm.Storage, msg *types.UnsignedMessage, bh abi.ChainEpoch) (*consensus.ApplicationResult, error) {
//	return applyTestMessageWithAncestors(vm.DefaultActors, st, store, msg, bh, nil)
//}
//
//// ApplyTestMessageWithActors sends a message directly to the vm with a given set of builtin actors
//func ApplyTestMessageWithActors(actors vm.ActorCodeLoader, st state.State, store vm.Storage, msg *types.UnsignedMessage, bh abi.ChainEpoch) (*consensus.ApplicationResult, error) {
//	return applyTestMessageWithAncestors(actors, st, store, msg, bh, nil)
//}
//
//// ApplyTestMessageWithGas uses the FakeBlockRewarder but the default SignedMessageValidator
//func ApplyTestMessageWithGas(actors vm.ActorCodeLoader, st state.State, store vm.Storage, msg *types.UnsignedMessage, bh abi.ChainEpoch, minerOwner address.Address) (*consensus.ApplicationResult, error) {
//	applier := consensus.NewConfiguredProcessor(actors, &vm.FakeSyscalls{}, &consensus.FakeChainRandomness{})
//	return newMessageApplier(msg, applier, st, store, bh, minerOwner, nil)
//}
//
//func newMessageApplier(msg *types.UnsignedMessage, processor *consensus.DefaultProcessor, st state.State, vms vm.Storage, bh abi.ChainEpoch, minerOwner address.Address, ancestors []block.TipSet) (*consensus.ApplicationResult, error) {
//	return nil, nil
//}
//
//// CreateAndApplyTestMessageFrom wraps the given parameters in a message and calls ApplyTestMessage.
//func CreateAndApplyTestMessageFrom(t *testing.T, st state.State, vms vm.Storage, from address.Address, to address.Address, val, bh uint64, method abi.MethodNum, ancestors []block.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
//	t.Helper()
//
//	pdata, err := encoding.Encode(params)
//	if err != nil {
//		panic(err)
//	}
//	msg := types.NewUnsignedMessage(from, to, 0, types.NewAttoFILFromFIL(val), method, pdata)
//	return applyTestMessageWithAncestors(vm.DefaultActors, st, vms, msg, abi.ChainEpoch(bh), ancestors)
//}
//
//// CreateAndApplyTestMessage wraps the given parameters in a message and calls
//// CreateAndApplyTestMessageFrom sending the message from address.TestAddress
//func CreateAndApplyTestMessage(t *testing.T, st state.State, vms vm.Storage, to address.Address, val, bh uint64, method abi.MethodNum, ancestors []block.TipSet, params ...interface{}) (*consensus.ApplicationResult, error) {
//	return CreateAndApplyTestMessageFrom(t, st, vms, address.TestAddress, to, val, bh, method, ancestors, params...)
//}
//
//func applyTestMessageWithAncestors(actors vm.ActorCodeLoader, st state.State, store vm.Storage, msg *types.UnsignedMessage, bh abi.ChainEpoch, ancestors []block.TipSet) (*consensus.ApplicationResult, error) {
//	msg.GasFeeCap = types.NewGasFeeCap(1)
//	msg.GasPremium = types.NewGasPremium(1)
//	msg.GasLimit = types.NewGas(300)
//
//	ta := newTestApplier(actors)
//	return newMessageApplier(msg, ta, st, store, bh, address.Undef, ancestors)
//}
//
//func newTestApplier(actors vm.ActorCodeLoader) *consensus.DefaultProcessor {
//	return consensus.NewConfiguredProcessor(actors, &vm.FakeSyscalls{}, &consensus.FakeChainRandomness{})
//}
