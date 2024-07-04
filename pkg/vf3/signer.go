package vf3

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/gpbft"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"golang.org/x/xerrors"
)

type signer struct {
	wallet v1api.IWallet
}

// Sign signs a message with the private key corresponding to a public key.
// The the key must be known by the wallet and be of BLS type.
func (s *signer) Sign(sender gpbft.PubKey, msg []byte) ([]byte, error) {
	addr, err := address.NewBLSAddress(sender)
	if err != nil {
		return nil, xerrors.Errorf("converting pubkey to address: %w", err)
	}
	sig, err := s.wallet.WalletSign(context.TODO(), addr, msg, types.MsgMeta{Type: types.MTUnknown})
	if err != nil {
		return nil, xerrors.Errorf("error while signing: %w", err)
	}
	return sig.Data, nil
}

// MarshalPayloadForSigning marshals the given payload into the bytes that should be signed.
// This should usually call `Payload.MarshalForSigning(NetworkName)` except when testing as
// that method is slow (computes a merkle tree that's necessary for testing).
// Implementations must be safe for concurrent use.
func (s *signer) MarshalPayloadForSigning(nn gpbft.NetworkName, p *gpbft.Payload) []byte {
	return p.MarshalForSigning(nn)
}
