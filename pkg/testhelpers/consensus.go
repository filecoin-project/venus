package testhelpers

import (
	"context"
	fbig "github.com/filecoin-project/go-state-types/big"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	bls "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
)

// RequireSignedTestBlockFromTipSet creates a block with a valid signature by
// the passed in miner work and a Miner field set to the minerAddr.
func RequireSignedTestBlockFromTipSet(t *testing.T, baseTipSet types.TipSet, stateRootCid cid.Cid, receiptRootCid cid.Cid, height abi.ChainEpoch, minerAddr address.Address, minerWorker address.Address, signer types.Signer) *types.BlockHeader {
	ticket := consensus.MakeFakeTicketForTest()
	emptyBLSSig := crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: (*bls.Aggregate([]bls.Signature{}))[:],
	}

	b := &types.BlockHeader{
		Miner:                 minerAddr,
		Ticket:                ticket,
		Parents:               baseTipSet.Key(),
		ParentWeight:          fbig.NewInt(int64(height * 10000)),
		Height:                height,
		ParentStateRoot:       stateRootCid,
		ParentMessageReceipts: receiptRootCid,
		BLSAggregate:          &emptyBLSSig,
	}
	sig, err := signer.SignBytes(context.TODO(), b.SignatureData(), minerWorker)
	require.NoError(t, err)
	b.BlockSig = sig

	return b
}

// FakeBlockValidator passes everything as valid
type FakeBlockValidator struct{}

// NewFakeBlockValidator createas a FakeBlockValidator that passes everything as valid.
func NewFakeBlockValidator() *FakeBlockValidator {
	return &FakeBlockValidator{}
}

// ValidateHeaderSemantic does nothing.
func (fbv *FakeBlockValidator) ValidateHeaderSemantic(ctx context.Context, child *types.BlockHeader, parents types.TipSet) error {
	return nil
}

// ValidateSyntax does nothing.
func (fbv *FakeBlockValidator) ValidateSyntax(ctx context.Context, blk *types.BlockHeader) error {
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
	syntaxStubs map[cid.Cid]error
}

// NewStubBlockValidator creates a StubBlockValidator that allows errors to configured
// for blocks passed to the Validate* methods.
func NewStubBlockValidator() *StubBlockValidator {
	return &StubBlockValidator{
		syntaxStubs: make(map[cid.Cid]error),
	}
}

// ValidateSyntax return nil or error for stubbed block `blk`.
func (mbv *StubBlockValidator) ValidateBlockMsg(ctx context.Context, blk *types.BlockMsg) pubsub.ValidationResult {
	if mbv.syntaxStubs[blk.Header.Cid()] == nil {
		return pubsub.ValidationAccept
	}
	return pubsub.ValidationReject
}

// StubSyntaxValidationForBlock stubs an error when the ValidateSyntax is called
// on the with the given block.
func (mbv *StubBlockValidator) StubSyntaxValidationForBlock(blk *types.BlockHeader, err error) {
	mbv.syntaxStubs[blk.Cid()] = err
}
