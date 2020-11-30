package wallet

import (
	"context"
	"errors"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/wallet"
)

var ErrNoDefaultFromAddress = errors.New("unable to determine a default wallet address")

type WalletAPI struct { //nolint
	wallet *WalletSubmodule
}

// WalletBalance returns the current balance of the given wallet address.
func (walletAPI *WalletAPI) WalletBalance(ctx context.Context, addr address.Address) (abi.TokenAmount, error) {
	act, err := walletAPI.wallet.Chain.API().GetActor(ctx, addr)
	if err == types.ErrActorNotFound {
		return abi.NewTokenAmount(0), nil
	} else if err != nil {
		return abi.NewTokenAmount(0), err
	}

	return act.Balance, nil
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) WalletDefaultAddress() (address.Address, error) {
	ret, err := walletAPI.wallet.Config.Get("wallet.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || !addr.Empty() {
		return addr, err
	}

	// No default is set; pick the 0th and make it the default.
	if len(walletAPI.WalletAddresses()) > 0 {
		addr := walletAPI.WalletAddresses()[0]
		err := walletAPI.wallet.Config.Set("wallet.defaultAddress", addr.String())
		if err != nil {
			return address.Undef, err
		}

		return addr, nil
	}

	return address.Undef, ErrNoDefaultFromAddress
}

// WalletAddresses gets addresses from the wallet
func (walletAPI *WalletAPI) WalletAddresses() []address.Address {
	return walletAPI.wallet.Wallet.Addresses()
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func (walletAPI *WalletAPI) SetWalletDefaultAddress(addr address.Address) error {
	localAddrs := walletAPI.WalletAddresses()
	for _, localAddr := range localAddrs {
		if localAddr == addr {
			err := walletAPI.wallet.Config.Set("wallet.defaultAddress", addr.String())
			if err != nil {
				return err
			}
			return nil
		}
	}
	return errors.New("addr not in the wallet list")
}

// WalletNewAddress generates a new wallet address
func (walletAPI *WalletAPI) WalletNewAddress(protocol address.Protocol) (address.Address, error) {
	return wallet.NewAddress(walletAPI.wallet.Wallet, protocol)
}

// WalletImport adds a given set of KeyInfos to the wallet
func (walletAPI *WalletAPI) WalletImport(kinfos ...*crypto.KeyInfo) ([]address.Address, error) {
	return walletAPI.wallet.Wallet.Import(kinfos...)
}

// WalletExport returns the KeyInfos for the given wallet addresses
func (walletAPI *WalletAPI) WalletExport(addrs []address.Address) ([]*crypto.KeyInfo, error) {
	return walletAPI.wallet.Wallet.Export(addrs)
}
