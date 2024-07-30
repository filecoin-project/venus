package vf3

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/gpbft"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type signer struct {
	wallet v1api.IWallet
}

// Sign signs a message with the private key corresponding to a public key.
// The the key must be known by the wallet and be of BLS type.
func (s *signer) Sign(ctx context.Context, sender gpbft.PubKey, msg []byte) ([]byte, error) {
	addr, err := address.NewBLSAddress(sender)
	if err != nil {
		return nil, fmt.Errorf("converting pubkey to address: %w", err)
	}
	sig, err := s.wallet.WalletSign(ctx, addr, msg, types.MsgMeta{Type: types.MTUnknown})
	if err != nil {
		return nil, fmt.Errorf("error while signing: %w", err)
	}
	return sig.Data, nil
}
