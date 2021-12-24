package market

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/statemanger"
	apitypes "github.com/filecoin-project/venus/venus-shared/api/chain"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	types "github.com/filecoin-project/venus/venus-shared/chain"
	"golang.org/x/xerrors"
)

type marketAPI struct {
	chain v1api.IChain
	stmgr statemanger.IStateManager
}

func newMarketAPI(c v1api.IChain, stmgr statemanger.IStateManager) v1api.IMarket {
	return &marketAPI{c, stmgr}
}

// StateMarketParticipants returns the Escrow and Locked balances of every participant in the Storage Market
func (m *marketAPI) StateMarketParticipants(ctx context.Context, tsk types.TipSetKey) (map[string]apitypes.MarketBalance, error) {
	out := map[string]apitypes.MarketBalance{}
	ts, err := m.chain.ChainGetTipSet(ctx, tsk)
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

		out[a.String()] = apitypes.MarketBalance{
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
