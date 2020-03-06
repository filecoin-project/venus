package consensus

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

func (v *SignatureValidator) ValidateMessageSignature(ctx context.Context, msg *types.SignedMessage) error {
	data, err := msg.Message.Marshal()
	if err != nil {
		return errors.Wrapf(err, "failed to encode message to check signature")
	}
	signerAddress, err := v.state.AccountSignerAddress(ctx, msg.Message.From)
	if err != nil {
		return errors.Wrapf(err, "failed to load signer address for %v", msg.Message.From)
	}
	return crypto.ValidateSignature(data, signerAddress, msg.Signature)
}

func (v *SignatureValidator) ValidateBLSMessageAggregate(ctx context.Context, msgs []*types.UnsignedMessage, sig crypto.Signature) error {
	pubKeys := [][]byte{}
	encodedMsgs := [][]byte{}
	for _, msg := range msgs {
		signerAddress, err := v.state.AccountSignerAddress(ctx, msg.From)
		if err != nil {
			return errors.Wrapf(err, "failed to load signer address for %v", msg.From)
		}
		pubKeys = append(pubKeys, signerAddress.Payload())
		msgBytes, err := msg.Marshal()
		if err != nil {
			return err
		}
		encodedMsgs = append(encodedMsgs, msgBytes)
	}
	if !crypto.VerifyBLSAggregate(pubKeys, encodedMsgs, sig.Data) {
		return errors.New("BLS signature invalid")
	}
	return nil
}
