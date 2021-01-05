package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/state"
	xerrors "github.com/pkg/errors"
)

type MarketAPI struct {
	chain *ChainSubmodule
}

func (marketAPI *MarketAPI) StateMarketBalance(ctx context.Context, addr address.Address, tsk block.TipSetKey) (state.MarketBalance, error) {
	view, err := marketAPI.chain.State.ParentStateView(tsk)
	if err != nil {
		return state.MarketBalance{}, xerrors.Errorf("loading view %s: %v", tsk, err)
	}
	return view.MarketBalance(ctx, addr)
}
