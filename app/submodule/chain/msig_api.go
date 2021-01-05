package chain

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/block"
	xerrors "github.com/pkg/errors"
)

type MsigAPI struct {
	chain *ChainSubmodule
}

func (mSigApi *MsigAPI) MsigGetAvailableBalance(ctx context.Context, addr address.Address, tsk block.TipSetKey) (big.Int, error) {
	if tsk.IsEmpty() {
		tsk = mSigApi.chain.State.Head()
	}
	ts, err := mSigApi.chain.State.GetTipSet(tsk)
	if err != nil {
		return big.NewInt(0), xerrors.Errorf("loading tipset %s: %v", tsk, err)
	}

	view, err := mSigApi.chain.State.StateView(tsk)
	if err != nil {
		return big.NewInt(0), xerrors.Errorf("loading state view %s: %v", tsk, err)
	}

	act, err := view.LoadActor(ctx, addr)
	if err != nil {
		return big.NewInt(0), xerrors.Errorf("loading actor %s: %v", tsk, err)
	}

	msas, err := view.LoadMSigState(ctx, addr)
	if err != nil {
		return big.NewInt(0), xerrors.Errorf("failed to load multisig actor state: %w", err)
	}

	locked, err := msas.LockedBalance(ts.EnsureHeight())
	if err != nil {
		return big.NewInt(0), xerrors.Errorf("failed to compute locked multisig balance: %w", err)
	}
	return big.Sub(act.Balance, locked), nil
}

func (mSigApi *MsigAPI) MsigGetVested(ctx context.Context, addr address.Address, start block.TipSetKey, end block.TipSetKey) (big.Int, error) {
	startTs, err := mSigApi.chain.State.GetTipSet(start)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading start tipset %s: %w", start, err)
	}

	endTs, err := mSigApi.chain.State.GetTipSet(end)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading end tipset %s: %w", end, err)
	}

	if startTs.EnsureHeight() > endTs.EnsureHeight() {
		return big.Int{}, xerrors.Errorf("start tipset %d is after end tipset %d", startTs.EnsureHeight(), endTs.EnsureHeight())
	} else if startTs.EnsureHeight() == endTs.EnsureHeight() {
		return big.Zero(), nil
	}

	view, err := mSigApi.chain.State.StateView(end)
	if err != nil {
		return big.Int{}, xerrors.Errorf("loading state view %s: %v", end, err)
	}

	msas, err := view.LoadMSigState(ctx, addr)
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to load multisig actor state: %w", err)
	}

	startLk, err := msas.LockedBalance(startTs.EnsureHeight())
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to compute locked balance at start height: %w", err)
	}

	endLk, err := msas.LockedBalance(endTs.EnsureHeight())
	if err != nil {
		return big.Int{}, xerrors.Errorf("failed to compute locked balance at end height: %w", err)
	}

	return big.Sub(startLk, endLk), nil
}
