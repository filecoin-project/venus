package state

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// SignatureValidator resolves account actor addresses to their pubkey-style address for signature validation.
type SignatureValidator struct {
	signerView AccountView
}

func NewSignatureValidator(signerView AccountView) *SignatureValidator {
	return &SignatureValidator{signerView: signerView}
}

// ValidateSignature check the signature is valid or not
func (v *SignatureValidator) ValidateSignature(ctx context.Context, data []byte, signer address.Address, sig crypto.Signature) error {
	signerAddress, err := v.signerView.ResolveToDeterministicAddress(ctx, signer)
	if err != nil {
		return errors.Wrapf(err, "failed to load signer address for %v", signer)
	}
	return crypto.Verify(&sig, signerAddress, data)
}

// ValidateBLSMessageAggregate validate bls aggregate message
func (v *SignatureValidator) ValidateBLSMessageAggregate(ctx context.Context, msgs []*types.Message, sig *crypto.Signature) error {
	if sig == nil {
		if len(msgs) > 0 {
			return errors.New("Invalid empty BLS sig over messages")
		}
		return nil
	}

	if len(msgs) == 0 {
		return nil
	}

	var pubKeys [][]byte
	var encodedMsgCids [][]byte
	for _, msg := range msgs {
		signerAddress, err := v.signerView.ResolveToDeterministicAddress(ctx, msg.From)
		if err != nil {
			return errors.Wrapf(err, "failed to load signer address for %v", msg.From)
		}
		pubKeys = append(pubKeys, signerAddress.Payload())
		mCid := msg.Cid()
		encodedMsgCids = append(encodedMsgCids, mCid.Bytes())
	}

	if crypto.VerifyAggregate(pubKeys, encodedMsgCids, sig.Data) != nil {
		return errors.New("BLS signature invalid")
	}
	return nil
}
