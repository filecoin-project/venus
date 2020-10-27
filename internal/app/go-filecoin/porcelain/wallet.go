package porcelain

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// ErrNoDefaultFromAddress is returned when a default wallet address couldn't be determined (eg, there are zero addresses in the wallet).
var ErrNoDefaultFromAddress = errors.New("unable to determine a default wallet address")

type wbPlumbing interface {
	ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error)
}

// WalletBalance gets the current balance associated with an address
func WalletBalance(ctx context.Context, plumbing wbPlumbing, addr address.Address) (abi.TokenAmount, error) {
	act, err := plumbing.ActorGet(ctx, addr)
	if err == types.ErrNotFound {
		// if the account doesn't exit, the balance should be zero
		return abi.NewTokenAmount(0), nil
	}
	if err != nil {
		return abi.NewTokenAmount(0), err
	}
	return act.Balance, nil
}

type wdaPlumbing interface {
	ConfigGet(dottedPath string) (interface{}, error)
	ConfigSet(dottedPath string, paramJSON string) error
	WalletAddresses() []address.Address
}

// WalletDefaultAddress returns a default wallet address from the config.
// If none is set it picks the first address in the wallet and
// sets it as the default in the config.
func WalletDefaultAddress(plumbing wdaPlumbing) (address.Address, error) {
	ret, err := plumbing.ConfigGet("wallet.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || !addr.Empty() {
		return addr, err
	}

	// No default is set; pick the 0th and make it the default.
	if len(plumbing.WalletAddresses()) > 0 {
		addr := plumbing.WalletAddresses()[0]
		err := plumbing.ConfigSet("wallet.defaultAddress", addr.String())
		if err != nil {
			return address.Undef, err
		}

		return addr, nil
	}

	return address.Undef, ErrNoDefaultFromAddress
}

// SetWalletDefaultAddress set the specified address as the default in the config.
func SetWalletDefaultAddress(plumbing wdaPlumbing, a address.Address) error {
	addrs := plumbing.WalletAddresses()

	for _, addr := range addrs {
		if addr == a {
			err := plumbing.ConfigSet("wallet.defaultAddress", a.String())
			if err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("addr not in the wallet list")
}
