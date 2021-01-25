package consensus

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/crypto"
	state2 "github.com/filecoin-project/venus/pkg/state"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

func (c *Expected) CallWithGas(ctx context.Context, msg *types.UnsignedMessage, priorMsgs []types.ChainMsg, ts *block.TipSet) (*vm.Ret, error) {
	var (
		err       error
		stateRoot cid.Cid
		height    abi.ChainEpoch
	)
	if ts == nil {
		ts = c.chainState.GetHead()
		// Search back till we find a height with no fork, or we reach the beginning.
		// We need the _previous_ height to have no fork, because we'll
		// run the fork logic in `sm.TipSetState`. We need the _current_
		// height to have no fork, because we'll run it inside this
		// function before executing the given message.
		height, err = ts.Height()
		if err != nil {
			return nil, err
		}
		for height > 0 && (c.fork.HasExpensiveFork(ctx, height) || c.fork.HasExpensiveFork(ctx, height-1)) {
			ts, err = c.chainState.GetTipSet(ts.EnsureParents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}

		stateRoot, err = c.chainState.GetTipSetStateRoot(c.chainState.GetHead())
		if err != nil {
			return nil, err
		}
	} else {
		stateRoot, err = c.chainState.GetTipSetStateRoot(ts)
		if err != nil {
			return nil, err
		}
	}

	// When we're not at the genesis block, make sure we don't have an expensive migration.
	height, err = ts.Height()
	if err != nil {
		return nil, err
	}
	if height > 0 && (c.fork.HasExpensiveFork(ctx, height) || c.fork.HasExpensiveFork(ctx, height-1)) {
		return nil, fork.ErrExpensiveFork
	}

	priorState, err := state.LoadState(ctx, cbor.NewCborStore(c.bstore), stateRoot)
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
		Epoch:             height + 1,
		GasPriceSchedule:  c.gasPirceSchedule,
		PRoot:             stateRoot,
		Bsstore:           c.bstore,
		SysCallsImpl:      c.syscallsImpl,
		Fork:              c.fork,
	}

	for i, m := range priorMsgs {
		_, err := c.processor.ProcessMessage(ctx, m, vmOption)
		if err != nil {
			return nil, xerrors.Errorf("applying prior message (%d): %v", i, err)
		}
	}

	fromActor, found, err := priorState.GetActor(ctx, msg.VMMessage().From)
	if err != nil {
		return nil, xerrors.Errorf("get actor failed: %s", err)
	}
	if !found {
		return nil, xerrors.New("actor not found")
	}
	msg.Nonce = fromActor.Nonce

	viewer := state2.NewView(c.cstore, stateRoot)
	fromKey, err := viewer.ResolveToKeyAddr(ctx, msg.VMMessage().From)
	if err != nil {
		return nil, err
	}

	var msgApply types.ChainMsg
	switch fromKey.Protocol() {
	case address.BLS:
		msgApply = msg
	case address.SECP256K1:
		msgApply = &types.SignedMessage{
			Message: *msg,
			Signature: acrypto.Signature{
				Type: crypto.SigTypeSecp256k1,
				Data: make([]byte, 65),
			},
		}
	}
	return c.processor.ProcessMessage(ctx, msgApply, vmOption)
}

func (c *Expected) Call(ctx context.Context, msg *types.UnsignedMessage, ts *block.TipSet) (*vm.Ret, error) {
	ctx, span := trace.StartSpan(ctx, "statemanager.Call")
	defer span.End()
	chainReader := c.chainState
	// If no tipset is provided, try to find one without a fork.
	var err error
	if ts == nil {
		ts = chainReader.GetHead()

		// Search back till we find a height with no fork, or we reach the beginning.
		for ts.EnsureHeight() > 0 && c.fork.HasExpensiveFork(ctx, ts.EnsureHeight()-1) {
			var err error
			ts, err = chainReader.GetTipSet(ts.EnsureParents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}
	bstate := ts.At(0).ParentStateRoot
	bheight := ts.EnsureHeight()

	// If we have to run an expensive migration, and we're not at genesis,
	// return an error because the migration will take too long.
	//
	// We allow this at height 0 for at-genesis migrations (for testing).
	if bheight-1 > 0 && c.fork.HasExpensiveFork(ctx, bheight-1) {
		return nil, ErrExpensiveFork
	}

	// Run the (not expensive) migration.
	bstate, err = c.fork.HandleStateForks(ctx, bstate, bheight-1, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to handle fork: %v", err)
	}

	rnd := HeadRandomness{
		Chain: c.rnd,
		Head:  ts.Key(),
	}

	if msg.GasLimit == 0 {
		msg.GasLimit = constants.BlockGasLimit
	}

	st, err := state.LoadState(ctx, cbor.NewCborStore(c.bstore), bstate)
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
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               &rnd,
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             ts.At(0).Height,
		GasPriceSchedule:  c.gasPirceSchedule,
		Fork:              c.fork,
		PRoot:             ts.At(0).ParentStateRoot,
		Bsstore:           c.bstore,
		SysCallsImpl:      c.syscallsImpl,
	}

	// TODO: maybe just use the invoker directly?
	return c.processor.ProcessMessage(ctx, msg, vmOption)
}
