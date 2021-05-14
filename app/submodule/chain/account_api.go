package chain

import (
	"context"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/go-address"
	xerrors "github.com/pkg/errors"
)

type IAccount interface {
	StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error)
}

var _ IAccount = &AccountAPI{}

type AccountAPI struct {
	chain *ChainSubmodule
}

func NewAccountAPI(chain *ChainSubmodule) AccountAPI {
	return AccountAPI{chain: chain}
}

func (accountAPI *AccountAPI) StateAccountKey(ctx context.Context, addr address.Address, tsk types.TipSetKey) (address.Address, error) {
	ts, err := accountAPI.chain.ChainReader.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := accountAPI.chain.ChainReader.StateView(ts)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.ResolveToKeyAddr(ctx, addr)
}
