package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/state"
	xerrors "github.com/pkg/errors"
	"go.opencensus.io/trace"
)

func (c *Expected) CallWithGas(ctx context.Context, msg *types.UnsignedMessage) (*vm.Ret, error) {
	head := c.chainState.GetHead()
	stateRoot, err := c.chainState.GetTipSetStateRoot(head)
	if err != nil {
		return nil, err
	}

	ts, err := c.chainState.GetTipSet(head)
	if err != nil {
		return nil, err
	}

	vms := vm.NewStorage(c.bstore)
	priorState, err := state.LoadState(ctx, vms, stateRoot)
	if err != nil {
		return nil, err
	}

	rnd := HeadRandomness{
		Chain: c.rnd,
		Head:  ts.Key(),
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree state.Tree) (abi.TokenAmount, error) {
			dertail, err := c.chainState.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             ts.At(0).Height,
		GasPriceSchedule:  c.gasPirceSchedule,
	}
	return c.processor.ProcessUnsignedMessage(ctx, msg, priorState, vms, vmOption)
}

func (c *Expected) Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*vm.Ret, error) {
	ctx, span := trace.StartSpan(ctx, "statemanager.Call")
	defer span.End()
	chainReader := c.chainState
	fork := c.fork
	// If no tipset is provided, try to find one without a fork.
	var err error
	if ts == nil {
		tsKey := chainReader.GetHead()
		ts, err = chainReader.GetTipSet(tsKey)
		if err != nil {
			return nil, xerrors.Errorf("failed to find TipSet: %v %v", tsKey, err)
		}

		// Search back till we find a height with no fork, or we reach the beginning.
		for ts.EnsureHeight() > 0 && fork.HasExpensiveFork(ctx, ts.EnsureHeight()-1) {
			var err error
			ts, err = chainReader.GetTipSet(ts.EnsureParents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}
	/*	vms := vm.NewStorage(c.bstore)
		bstate, err := state.LoadState(ctx, vms, ts.At(0).ParentStateRoot.Cid)
		if err != nil {
			return nil, err
		}*/
	bstate := ts.At(0).ParentStateRoot.Cid
	bheight := ts.EnsureHeight()

	// If we have to run an expensive migration, and we're not at genesis,
	// return an error because the migration will take too long.
	//
	// We allow this at height 0 for at-genesis migrations (for testing).
	if bheight-1 > 0 && fork.HasExpensiveFork(ctx, bheight-1) {
		return nil, ErrExpensiveFork
	}

	// Run the (not expensive) migration.
	bstate, err = fork.HandleStateForks(ctx, bstate, bheight-1, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to handle fork: %w", err)
	}

	rnd := HeadRandomness{
		Chain: c.rnd,
		Head:  ts.Key(),
	}

	if msg.GasLimit == 0 {
		msg.GasLimit = constants.BlockGasLimit
	}

	vms := vm.NewStorage(c.bstore)
	st, err := state.LoadState(ctx, vms, bstate)
	if err != nil {
		return nil, xerrors.Errorf("loading state: %v", err)
	}
	fromActor, found, err := st.GetActor(ctx, msg.From)
	if err != nil || !found {
		return nil, xerrors.Errorf("call raw get actor: %s", err)
	}

	msg.Nonce = fromActor.Nonce

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree state.Tree) (abi.TokenAmount, error) {
			dertail, err := chainReader.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             ts.At(0).Height,
		GasPriceSchedule:  c.gasPirceSchedule,
	}

	// TODO: maybe just use the invoker directly?
	return c.processor.ProcessUnsignedMessage(ctx, msg, st, vms, vmOption)
}
