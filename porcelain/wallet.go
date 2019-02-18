package porcelain

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

type walletPlumbing interface {
	ChainLs(ctx context.Context) <-chan interface{}
	ChainLatestState(ctx context.Context) (state.Tree, error)
}

// WalletBalance gets the current balance of the wallet
func WalletBalance(ctx context.Context, plumbing walletPlumbing, addr address.Address) (*types.AttoFIL, error) {
	tree, err := plumbing.ChainLatestState(ctx)
	if err != nil {
		return types.ZeroAttoFIL, err
	}

	act, err := tree.GetActor(ctx, addr)
	if err != nil {
		if state.IsActorNotFoundError(err) {
			// if the account doesn't exit, the balance should be zero
			return types.NewAttoFILFromFIL(0), nil
		}

		return types.ZeroAttoFIL, err
	}

	return act.Balance, nil
}
