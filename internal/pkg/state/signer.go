package state

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/wallet"
)

type chainHeadTracker interface {
	GetHead() block.TipSetKey
}

// Signer looks up non-signing addresses before signing
type Signer struct {
	viewer    *TipSetStateViewer
	chainHead chainHeadTracker
	wallet    *wallet.Wallet
}

// NewSigner creates a new signer
func NewSigner(viewer *TipSetStateViewer, chainHead chainHeadTracker, wallet *wallet.Wallet) *Signer {
	return &Signer{
		viewer:    viewer,
		chainHead: chainHead,
		wallet:    wallet,
	}
}

// SignBytes creates a signature for the given data using either the given addr or its associated signing address
func (s *Signer) SignBytes(ctx context.Context, data []byte, addr address.Address) (crypto.Signature, error) {
	signingAddr, err := s.signingAddress(ctx, addr)
	if err != nil {
		return crypto.Signature{}, err
	}
	return s.wallet.SignBytes(data, signingAddr)
}

// HasAddress returns whether this signer can sign with the given address
func (s *Signer) HasAddress(ctx context.Context, addr address.Address) (bool, error) {
	signingAddr, err := s.signingAddress(ctx, addr)
	if err != nil {
		return false, err
	}
	return s.wallet.HasAddress(signingAddr), nil
}

func (s *Signer) signingAddress(ctx context.Context, addr address.Address) (address.Address, error) {
	if addr.Protocol() == address.BLS || addr.Protocol() == address.SECP256K1 {
		// address is already a signing address. return it
		return addr, nil
	}

	view, err := s.viewer.StateView(s.chainHead.GetHead())
	if err != nil {
		return address.Undef, err
	}
	return view.AccountSignerAddress(ctx, addr)
}
