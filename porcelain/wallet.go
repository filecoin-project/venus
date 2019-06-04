package porcelain

import (
	"context"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// ErrNoDefaultFromAddress is returned when a default wallet address couldn't be determined (eg, there are zero addresses in the wallet).
var ErrNoDefaultFromAddress = errors.New("unable to determine a default wallet address")

type wbPlumbing interface {
	ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error)
}

// WalletBalance gets the current balance associated with an address
func WalletBalance(ctx context.Context, plumbing wbPlumbing, addr address.Address) (types.AttoFIL, error) {
	act, err := plumbing.ActorGet(ctx, addr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			// if the account doesn't exit, the balance should be zero
			return types.NewAttoFILFromFIL(0), nil
		}

		return types.ZeroAttoFIL, err
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
