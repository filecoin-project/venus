package series

import (
	"context"
	"errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// ErrWithWalletRestoreFailed is returned if the original address could not be restored.
var ErrWithWalletRestoreFailed = errors.New("failed to restore default wallet after WithWallet exited")

// WithWallet can be used to temporarlly change the default wallet address of
// the node to sessionWallet for all FAST actions executed inside of sessionFn.
//
// WithWallet should be used when you want to temporarally change the default
// wallet address of the node.
//
// Error ErrWithWalletRestoreFailed will be returned if the original address
// could not be restored.
func WithWallet(ctx context.Context, fc *fast.Filecoin, sessionWallet address.Address, sessionFn func(*fast.Filecoin) error) (err error) {
	var beforeAddress address.Address
	if err = fc.ConfigGet(ctx, "wallet.defaultAddress", &beforeAddress); err != nil {
		return
	}

	if err = fc.ConfigSet(ctx, "wallet.defaultAddress", sessionWallet); err != nil {
		return
	}

	defer func() {
		err = fc.ConfigSet(ctx, "wallet.defaultAddress", beforeAddress)
		if err != nil {
			err = ErrWithWalletRestoreFailed
		}
	}()

	return sessionFn(fc)
}
