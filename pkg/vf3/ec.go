package vf3

import (
	"context"
	"sort"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"
	"github.com/filecoin-project/venus/venus-shared/types"
	"golang.org/x/xerrors"
)

type ecWrapper struct {
	ChainStore   *chain.Store
	StateManager *statemanger.Stmgr
	Manifest     f3.Manifest
}

type f3TipSet types.TipSet

func (ts *f3TipSet) cast() *types.TipSet {
	return (*types.TipSet)(ts)
}

func (ts *f3TipSet) Key() gpbft.TipSetKey {
	return ts.cast().Key().Bytes()
}

func (ts *f3TipSet) Beacon() []byte {
	entries := ts.cast().Blocks()[0].BeaconEntries
	if len(entries) == 0 {
		// Set beacon to a non-nil slice to force the message builder to generate a
		// ticket. Otherwise, messages that require ticket, i.e. CONVERGE will fail
		// validation due to the absence of ticket. This is a convoluted way of doing it.
		// TODO: Rework the F3 message builder APIs to include ticket when needed instead
		//       of relying on the nil check of beacon.
		return []byte{}
	}
	return entries[len(entries)-1].Data
}

func (ts *f3TipSet) Epoch() int64 {
	return int64(ts.cast().Height())
}

func (ts *f3TipSet) Timestamp() time.Time {
	return time.Unix(int64(ts.cast().Blocks()[0].Timestamp), 0)
}

func wrapTS(ts *types.TipSet) f3.TipSet {
	if ts == nil {
		return nil
	}
	return (*f3TipSet)(ts)
}

// GetTipsetByEpoch should return a tipset before the one requested if the requested
// tipset does not exist due to null epochs
func (ec *ecWrapper) GetTipsetByEpoch(ctx context.Context, epoch int64) (f3.TipSet, error) {
	ts, err := ec.ChainStore.GetTipSetByHeight(ctx, nil, abi.ChainEpoch(epoch), true)
	if err != nil {
		return nil, xerrors.Errorf("getting tipset by height: %w", err)
	}
	return wrapTS(ts), nil
}

func (ec *ecWrapper) GetTipset(ctx context.Context, tsk gpbft.TipSetKey) (f3.TipSet, error) {
	tskLotus, err := types.TipSetKeyFromBytes(tsk)
	if err != nil {
		return nil, xerrors.Errorf("decoding tsk: %w", err)
	}

	ts, err := ec.ChainStore.GetTipSet(ctx, tskLotus)
	if err != nil {
		return nil, xerrors.Errorf("getting tipset by key: %w", err)
	}

	return wrapTS(ts), nil
}

func (ec *ecWrapper) GetHead(_ context.Context) (f3.TipSet, error) {
	return wrapTS(ec.ChainStore.GetHead()), nil
}

func (ec *ecWrapper) GetParent(ctx context.Context, tsF3 f3.TipSet) (f3.TipSet, error) {
	var ts *types.TipSet
	if tsW, ok := tsF3.(*f3TipSet); ok {
		ts = tsW.cast()
	} else {
		// There are only two implementations of F3.TipSet: f3TipSet, and one in fake EC
		// backend.
		//
		// TODO: Revisit the type check here and remove as needed once testing
		//       is over and fake EC backend goes away.
		tskLotus, err := types.TipSetKeyFromBytes(tsF3.Key())
		if err != nil {
			return nil, xerrors.Errorf("decoding tsk: %w", err)
		}
		ts, err = ec.ChainStore.GetTipSet(ctx, tskLotus)
		if err != nil {
			return nil, xerrors.Errorf("getting tipset by key for get parent: %w", err)
		}
	}
	parentTs, err := ec.ChainStore.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, xerrors.Errorf("getting parent tipset: %w", err)
	}
	return wrapTS(parentTs), nil
}

func (ec *ecWrapper) GetPowerTable(ctx context.Context, tskF3 gpbft.TipSetKey) (gpbft.PowerEntries, error) {
	tskLotus, err := types.TipSetKeyFromBytes(tskF3)
	if err != nil {
		return nil, xerrors.Errorf("decoding tsk: %w", err)
	}
	ts, err := ec.ChainStore.GetTipSet(ctx, tskLotus)
	if err != nil {
		return nil, xerrors.Errorf("getting tipset by key for get parent: %w", err)
	}

	_, state, err := ec.StateManager.ParentState(ctx, ts)
	if err != nil {
		return nil, xerrors.Errorf("loading the state tree: %w", err)
	}
	powerAct, _, err := state.GetActor(ctx, power.Address)
	if err != nil {
		return nil, xerrors.Errorf("getting the power actor: %w", err)
	}

	powerState, err := power.Load(ec.ChainStore.Store(ctx), powerAct)
	if err != nil {
		return nil, xerrors.Errorf("loading power actor state: %w", err)
	}

	var powerEntries gpbft.PowerEntries
	err = powerState.ForEachClaim(func(minerAddr address.Address, claim power.Claim) error {
		if claim.QualityAdjPower.Sign() <= 0 {
			return nil
		}

		// TODO: optimize
		ok, err := powerState.MinerNominalPowerMeetsConsensusMinimum(minerAddr)
		if err != nil {
			return xerrors.Errorf("checking consensus minimums: %w", err)
		}
		if !ok {
			return nil
		}

		id, err := address.IDFromAddress(minerAddr)
		if err != nil {
			return xerrors.Errorf("transforming address to ID: %w", err)
		}

		pe := gpbft.PowerEntry{
			ID:    gpbft.ActorID(id),
			Power: claim.QualityAdjPower.Int,
		}

		act, _, err := state.GetActor(ctx, minerAddr)
		if err != nil {
			return xerrors.Errorf("(get sset) failed to load miner actor: %w", err)
		}
		mstate, err := miner.Load(ec.ChainStore.Store(ctx), act)
		if err != nil {
			return xerrors.Errorf("(get sset) failed to load miner actor state: %w", err)
		}

		info, err := mstate.Info()
		if err != nil {
			return xerrors.Errorf("failed to load actor info: %w", err)
		}
		// check fee debt
		if debt, err := mstate.FeeDebt(); err != nil {
			return err
		} else if !debt.IsZero() {
			// fee debt don't add the miner to power table
			return nil
		}
		// check consensus faults
		if ts.Height() <= info.ConsensusFaultElapsed {
			return nil
		}

		waddr, err := vm.ResolveToDeterministicAddress(ctx, state, info.Worker, ec.ChainStore.Store(ctx))
		if err != nil {
			return xerrors.Errorf("resolve miner worker address: %w", err)
		}

		if waddr.Protocol() != address.BLS {
			return xerrors.Errorf("wrong type of worker address")
		}
		pe.PubKey = gpbft.PubKey(waddr.Payload())
		powerEntries = append(powerEntries, pe)
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("collecting the power table: %w", err)
	}

	sort.Sort(powerEntries)
	return powerEntries, nil
}
