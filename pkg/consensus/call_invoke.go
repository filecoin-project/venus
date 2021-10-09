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

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm"
)

//CallWithGas used to estimate message gaslimit, for each incoming message ,should execute after priorMsg in mpool
func (c *Expected) CallWithGas(ctx context.Context, msg *types.UnsignedMessage, priorMsgs []types.ChainMsg, ts *types.TipSet) (*vm.Ret, error) {
	var (
		err       error
		stateRoot cid.Cid
	)

	if ts == nil {
		ts = c.chainState.GetHead()

		// Search back till we find a height with no fork, or we reach the beginning.
		// We need the _previous_ height to have no fork, because we'll
		// run the fork logic in `sm.TipSetState`. We need the _current_
		// height to have no fork, because we'll run it inside this
		// function before executing the given message.
		for ts.Height() > 0 && (c.fork.HasExpensiveFork(ctx, ts.Height()) || c.fork.HasExpensiveFork(ctx, ts.Height()-1)) {
			ts, err = c.chainState.GetTipSet(ts.Parents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}

	// When we're not at the genesis block, make sure we don't have an expensive migration.
	if ts.Height() > 0 && (c.fork.HasExpensiveFork(ctx, ts.Height()) || c.fork.HasExpensiveFork(ctx, ts.Height()-1)) {
		return nil, fork.ErrExpensiveFork
	}

	stateRoot, err = c.chainState.GetTipSetStateRoot(ts)
	if err != nil {
		return nil, err
	}

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			cs, err := c.chainState.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return cs.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               NewHeadRandomness(c.rnd, ts.Key()),
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             ts.Height(),
		GasPriceSchedule:  c.gasPirceSchedule,
		PRoot:             stateRoot,
		Bsstore:           c.bstore,
		SysCallsImpl:      c.syscallsImpl,
		Fork:              c.fork,
	}

	vmi, err := vm.NewVM(vmOption)
	if err != nil {
		return nil, err
	}

	for i, m := range priorMsgs {
		_, err := vmi.ApplyMessage(m)
		if err != nil {
			return nil, xerrors.Errorf("applying prior message (%d): %v", i, err)
		}
	}

	fromActor, found, err := vmi.StateTree().GetActor(ctx, msg.VMMessage().From)
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
		return nil, xerrors.Errorf("could not resolve key: %v", err)
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
	return vmi.ApplyMessage(msgApply)
}

//Call used for api invoke to compute a msg base on specify tipset, if the tipset is null, use latest tipset in db
func (c *Expected) Call(ctx context.Context, msg *types.UnsignedMessage, ts *types.TipSet) (*vm.Ret, error) {
	ctx, span := trace.StartSpan(ctx, "statemanager.Call")
	defer span.End()
	chainReader := c.chainState

	// If no tipset is provided, try to find one without a fork.
	var err error
	if ts == nil {
		ts = chainReader.GetHead()

		// Search back till we find a height with no fork, or we reach the beginning.
		for ts.Height() > 0 && c.fork.HasExpensiveFork(ctx, ts.Height()-1) {
			var err error
			ts, err = chainReader.GetTipSet(ts.Parents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}

	bstate := ts.At(0).ParentStateRoot
	pts, err := c.chainState.GetTipSet(ts.Parents())
	if err != nil {
		return nil, xerrors.Errorf("failed to load parent tipset: %v", err)
	}
	pheight := pts.Height()

	// If we have to run an expensive migration, and we're not at genesis,
	// return an error because the migration will take too long.
	//
	// We allow this at height 0 for at-genesis migrations (for testing).
	if pheight > 0 && c.fork.HasExpensiveFork(ctx, pheight) {
		return nil, ErrExpensiveFork
	}

	// Run the (not expensive) migration.
	bstate, err = c.fork.HandleStateForks(ctx, bstate, pheight, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to handle fork: %v", err)
	}

	if msg.GasLimit == 0 {
		msg.GasLimit = constants.BlockGasLimit
	}

	if msg.GasFeeCap == types.EmptyTokenAmount {
		msg.GasFeeCap = abi.NewTokenAmount(0)
	}

	if msg.GasPremium == types.EmptyTokenAmount {
		msg.GasPremium = abi.NewTokenAmount(0)
	}

	if msg.Value == types.EmptyTokenAmount {
		msg.Value = abi.NewTokenAmount(0)
	}

	st, err := tree.LoadState(ctx, cbor.NewCborStore(c.bstore), bstate)
	if err != nil {
		return nil, xerrors.Errorf("loading state: %v", err)
	}

	fromActor, found, err := st.GetActor(ctx, msg.From)
	if err != nil || !found {
		return nil, xerrors.Errorf("call raw get actor: %s", err)
	}

	msg.Nonce = fromActor.Nonce

	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			dertail, err := chainReader.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		NtwkVersionGetter: c.fork.GetNtwkVersion,
		Rnd:               NewHeadRandomness(c.rnd, ts.Key()),
		BaseFee:           ts.At(0).ParentBaseFee,
		Epoch:             pheight + 1,
		GasPriceSchedule:  c.gasPirceSchedule,
		Fork:              c.fork,
		PRoot:             ts.At(0).ParentStateRoot,
		Bsstore:           c.bstore,
		SysCallsImpl:      c.syscallsImpl,
	}

	return c.processor.ProcessImplicitMessage(ctx, msg, vmOption)
}
