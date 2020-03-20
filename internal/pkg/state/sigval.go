package state

import (
	"context"

	addr "github.com/filecoin-project/go-address"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

type AccountStateView interface {
	AccountSignerAddress(ctx context.Context, a addr.Address) (addr.Address, error)
}

//
// SignatureValidator resolves account actor addresses to their pubkey-style address for signature validation.
//
type SignatureValidator struct {
	state AccountStateView
}

func NewSignatureValidator(state AccountStateView) *SignatureValidator {
	return &SignatureValidator{state: state}
}

func (v *SignatureValidator) ValidateSignature(ctx context.Context, data []byte, signer addr.Address, sig crypto.Signature) error {
	signerAddress, err := v.state.AccountSignerAddress(ctx, signer)
	if err != nil {
		return errors.Wrapf(err, "failed to load signer address for %v", signer)
	}
	return crypto.ValidateSignature(data, signerAddress, sig)
}

func (v *SignatureValidator) ValidateMessageSignature(ctx context.Context, msg *types.SignedMessage) error {
	mCid, err := msg.Message.Cid()
	if err != nil {
		return errors.Wrapf(err, "failed to take cid of message to check signature")
	}
	data := mCid.Bytes()

	return v.ValidateSignature(ctx, data, msg.Message.From, msg.Signature)
}

func (v *SignatureValidator) ValidateBLSMessageAggregate(ctx context.Context, msgs []*types.UnsignedMessage, sig crypto.Signature) error {
	pubKeys := [][]byte{}
	encodedMsgCids := [][]byte{}
	for _, msg := range msgs {
		signerAddress, err := v.state.AccountSignerAddress(ctx, msg.From)
		if err != nil {
			return errors.Wrapf(err, "failed to load signer address for %v", msg.From)
		}
		pubKeys = append(pubKeys, signerAddress.Payload())
		mCid, err := msg.Cid()
		if err != nil {
			return err
		}
		msgBytes := mCid.Bytes()
		encodedMsgCids = append(encodedMsgCids, msgBytes)
	}

	if !crypto.VerifyBLSAggregate(pubKeys, encodedMsgCids, sig.Data) {
		return errors.New("BLS signature invalid")
	}
	return nil
}
