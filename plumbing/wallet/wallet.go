package wallet

import (
	"github.com/filecoin-project/go-filecoin/address"
	w "github.com/filecoin-project/go-filecoin/wallet"
)

// Wallet is a plumbing wrapper for the wallet
type Wallet struct {
	wallet *w.Wallet
}

// NewWallet returns a new Wallet
func NewWallet(wallet *w.Wallet) *Wallet {
	return &Wallet{wallet: wallet}
}

// Addresses returns addresses from the wallet
func (wallet *Wallet) Addresses() []address.Address {
	return wallet.wallet.Addresses()
}

// Find finds the backend for an address on the wallet
func (wallet *Wallet) Find(address address.Address) (w.Backend, error) {
	return wallet.wallet.Find(address)
}

// NewAddress makes a new address
func (wallet *Wallet) NewAddress() (address.Address, error) {
	return w.NewAddress(wallet.wallet)
}
