package chain

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/block"
	xerrors "github.com/pkg/errors"
)

type AccountAPI struct {
	chain *ChainSubmodule
}

func NewAccountAPI(chain *ChainSubmodule) AccountAPI {
	return AccountAPI{chain: chain}
}

func (accountAPI *AccountAPI) StateAccountKey(ctx context.Context, addr address.Address, tsk block.TipSetKey) (address.Address, error) {
	ts, err := accountAPI.chain.State.GetTipSet(tsk)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := accountAPI.chain.State.StateView(ts)
	if err != nil {
		return address.Undef, xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	return view.ResolveToKeyAddr(ctx, addr)
}
