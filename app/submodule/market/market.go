package market

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
)

type IMarket interface {
	StateMarketParticipants(ctx context.Context, tsk block.TipSetKey) (map[string]chain.MarketBalance, error)
}
type marketAPI struct {
	chain chain.IChain
	stmgr statemanger.IStateManager
}

func newMarketAPI(c chain.IChain, stmgr statemanger.IStateManager) IMarket {
	return &marketAPI{c, stmgr}
}

func (m *marketAPI) StateMarketParticipants(ctx context.Context, tsk block.TipSetKey) (map[string]chain.MarketBalance, error) {
	out := map[string]chain.MarketBalance{}
	ts, err := m.chain.ChainGetTipSet(tsk)
	if err != nil {
		return nil, xerrors.Errorf("loading tipset %s: %w", tsk, err)
	}

	state, err := m.stmgr.GetMarketState(ctx, ts)
	if err != nil {
		return nil, err
	}
	escrow, err := state.EscrowTable()
	if err != nil {
		return nil, err
	}
	locked, err := state.LockedTable()
	if err != nil {
		return nil, err
	}

	err = escrow.ForEach(func(a address.Address, es abi.TokenAmount) error {

		lk, err := locked.Get(a)
		if err != nil {
			return err
		}

		out[a.String()] = chain.MarketBalance{
			Escrow: es,
			Locked: lk,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
