package porcelain

import (
	"context"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// ErrNoDefaultWalletAddress is returned when there is no default wallet address.
var ErrNoDefaultWalletAddress = errors.New("there is no default wallet address")

type walletPlumbing interface {
	ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error)
	ConfigGet(dottedPath string) (interface{}, error)
}

// WalletBalance gets the current balance associated with an address
func WalletBalance(ctx context.Context, plumbing walletPlumbing, addr address.Address) (*types.AttoFIL, error) {
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

// DefaultWalletAddress returns a default wallet address from the config.
func DefaultWalletAddress(plumbing walletPlumbing) (address.Address, error) {
	ret, err := plumbing.ConfigGet("wallet.defaultAddress")
	addr := ret.(address.Address)
	if err != nil || addr != (address.Address{}) {
		return addr, err
	}

	return address.Address{}, ErrNoDefaultWalletAddress
}
