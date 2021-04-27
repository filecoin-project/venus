package state

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

//todo remove Account view a nd headsignerview
type AccountView interface {
	ResolveToKeyAddr(ctx context.Context, address address.Address) (address.Address, error)
}

type tipSignerView interface {
	GetHead() *types.TipSet
	ResolveToKeyAddr(ctx context.Context, ts *types.TipSet, address address.Address) (address.Address, error)
}

// Signer looks up non-signing addresses before signing
type Signer struct {
	wallet     *wallet.Wallet
	signerView AccountView
}

// NewSigner creates a new signer
func NewSigner(signerView AccountView, wallet *wallet.Wallet) *Signer {
	return &Signer{
		signerView: signerView,
		wallet:     wallet,
	}
}

// SignBytes creates a signature for the given data using either the given addr or its associated signing address
func (s *Signer) SignBytes(ctx context.Context, data []byte, addr address.Address) (*crypto.Signature, error) {
	signingAddr, err := s.signerView.ResolveToKeyAddr(ctx, addr)
	if err != nil {
		return nil, err
	}
	return s.wallet.SignBytes(data, signingAddr)
}

// HasAddress returns whether this signer can sign with the given address
func (s *Signer) HasAddress(ctx context.Context, addr address.Address) (bool, error) {
	signingAddr, err := s.signerView.ResolveToKeyAddr(ctx, addr)
	if err != nil {
		return false, err
	}
	return s.wallet.HasAddress(signingAddr), nil
}

type HeadSignView struct {
	tipSignerView
}

func NewHeadSignView(tipSignerView tipSignerView) *HeadSignView {
	return &HeadSignView{tipSignerView: tipSignerView}
}

func (headSignView *HeadSignView) ResolveToKeyAddr(ctx context.Context, addr address.Address) (address.Address, error) {
	head := headSignView.GetHead()
	return headSignView.tipSignerView.ResolveToKeyAddr(ctx, head, addr) //nil will use latest
}
