package porcelain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type walletPlumbing interface {
	ActorGet(ctx context.Context, addr address.Address) (*actor.Actor, error)
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
