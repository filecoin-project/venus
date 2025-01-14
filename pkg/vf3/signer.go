package vf3

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/venus/pkg/wallet"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type signer struct {
	sign wallet.WalletSignFunc
}

// Sign signs a message with the private key corresponding to a public key.
// The key must be known by the wallet and be of BLS type.
func (s *signer) Sign(ctx context.Context, sender gpbft.PubKey, msg []byte) ([]byte, error) {
	addr, err := address.NewBLSAddress(sender)
	if err != nil {
		return nil, fmt.Errorf("converting pubkey to address: %w", err)
	}
	sig, err := s.sign(ctx, addr, msg, types.MsgMeta{Type: types.MTF3})
	if err != nil {
		return nil, fmt.Errorf("error while signing: %w", err)
	}
	return sig.Data, nil
}
