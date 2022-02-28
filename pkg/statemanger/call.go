package statemanger

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/state"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
)

// CallWithGas used to estimate message gaslimit, for each incoming message ,should execute after priorMsg in mpool
func (s *Stmgr) CallWithGas(ctx context.Context, msg *types.Message, priorMsgs []types.ChainMsg, ts *types.TipSet) (*vm.Ret, error) {
	var (
		err       error
		stateRoot cid.Cid
		view      *state.View
	)

	if ts == nil {
		ts = s.cs.GetHead()
		// Search back till we find a height with no fork, or we reach the beginning.
		// We need the _previous_ height to have no fork, because we'll
		// run the fork logic in `sm.TipSetState`. We need the _current_
		// height to have no fork, because we'll run it inside this
		// function before executing the given message.
		for ts.Height() > 0 && (s.fork.HasExpensiveFork(ctx, ts.Height()) || s.fork.HasExpensiveFork(ctx, ts.Height()-1)) {
			ts, err = s.cs.GetTipSet(ctx, ts.Parents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}

	// When we're not at the genesis block, make sure we don't have an expensive migration.
	if ts.Height() > 0 && (s.fork.HasExpensiveFork(ctx, ts.Height()) || s.fork.HasExpensiveFork(ctx, ts.Height()-1)) {
		return nil, fork.ErrExpensiveFork
	}

	if stateRoot, view, err = s.StateView(ctx, ts); err != nil {
		return nil, err
	}

	vmHeight := ts.Height() + 1

	filVested, err := s.cs.GetFilVested(ctx, vmHeight)
	if err != nil {
		return nil, err
	}

	buffStore := blockstoreutil.NewBufferedBstore(s.cs.Blockstore())
	vmOption := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			cs, err := s.cs.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return cs.FilCirculating, nil
		},
		LookbackStateGetter: vmcontext.LookbackStateGetterForTipset(ctx, s.cs, s.fork, ts),
		FilVested:           filVested,
		NetworkVersion:      s.fork.GetNetworkVersion(ctx, ts.Height()+1),
		Rnd:                 consensus.NewHeadRandomness(s.rnd, ts.Key()),
		BaseFee:             ts.At(0).ParentBaseFee,
		Epoch:               vmHeight,
		GasPriceSchedule:    s.gasSchedule,
		PRoot:               stateRoot,
		Bsstore:             buffStore,
		SysCallsImpl:        s.syscallsImpl,
		Fork:                s.fork,
	}

	vmi, err := vm.NewVM(ctx, vmOption)
	if err != nil {
		return nil, err
	}

	for i, m := range priorMsgs {
		_, err := vmi.ApplyMessage(ctx, m)
		if err != nil {
			return nil, xerrors.Errorf("applying prior message (%d): %v", i, err)
		}
	}

	stateRoot, err = vmi.Flush(ctx)
	if err != nil {
		return nil, xerrors.Errorf("flushing vm: %w", err)
	}

	stTree, err := tree.LoadState(ctx, cbor.NewCborStore(buffStore), stateRoot)
	if err != nil {
		return nil, xerrors.Errorf("loading state tree: %w", err)
	}

	fromActor, found, err := stTree.GetActor(ctx, msg.VMMessage().From)
	if err != nil {
		return nil, xerrors.Errorf("get actor failed: %s", err)
	}
	if !found {
		return nil, xerrors.New("actor not found")
	}
	msg.Nonce = fromActor.Nonce

	fromKey, err := view.ResolveToKeyAddr(ctx, msg.VMMessage().From)
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
	return vmi.ApplyMessage(ctx, msgApply)
}

// Call used for api invoke to compute a msg base on specify tipset, if the tipset is null, use latest tipset in db
func (s *Stmgr) Call(ctx context.Context, msg *types.Message, ts *types.TipSet) (*vm.Ret, error) {
	ctx, span := trace.StartSpan(ctx, "statemanager.Call")
	defer span.End()

	// If no tipset is provided, try to find one without a fork.
	var err error
	if ts == nil {
		ts = s.cs.GetHead()

		// Search back till we find a height with no fork, or we reach the beginning.
		for ts.Height() > 0 && s.fork.HasExpensiveFork(ctx, ts.Height()-1) {
			var err error
			ts, err = s.cs.GetTipSet(ctx, ts.Parents())
			if err != nil {
				return nil, xerrors.Errorf("failed to find a non-forking epoch: %v", err)
			}
		}
	}

	pts, err := s.cs.GetTipSet(ctx, ts.Parents())
	if err != nil {
		return nil, xerrors.Errorf("failed to load parent tipset: %v", err)
	}

	bstate := ts.At(0).ParentStateRoot
	pheight := pts.Height()
	vmHeight := pheight + 1

	// If we have to run an expensive migration, and we're not at genesis,
	// return an error because the migration will take too long.
	//
	// We allow this at height 0 for at-genesis migrations (for testing).
	if pheight > 0 && s.fork.HasExpensiveFork(ctx, pheight) {
		return nil, consensus.ErrExpensiveFork
	}

	// Run the (not expensive) migration.
	bstate, err = s.fork.HandleStateForks(ctx, bstate, pheight, ts)
	if err != nil {
		return nil, fmt.Errorf("failed to handle fork: %v", err)
	}

	filVested, err := s.cs.GetFilVested(ctx, vmHeight)
	if err != nil {
		return nil, err
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

	st, err := tree.LoadState(ctx, cbor.NewCborStore(s.cs.Blockstore()), bstate)
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
			dertail, err := s.cs.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return dertail.FilCirculating, nil
		},
		LookbackStateGetter: vmcontext.LookbackStateGetterForTipset(ctx, s.cs, s.fork, ts),
		FilVested:           filVested,
		NetworkVersion:      s.fork.GetNetworkVersion(ctx, pheight+1),
		Rnd:                 consensus.NewHeadRandomness(s.rnd, ts.Key()),
		BaseFee:             ts.At(0).ParentBaseFee,
		Epoch:               vmHeight,
		GasPriceSchedule:    s.gasSchedule,
		Fork:                s.fork,
		PRoot:               ts.At(0).ParentStateRoot,
		Bsstore:             s.cs.Blockstore(),
		SysCallsImpl:        s.syscallsImpl,
	}

	v, err := vm.NewVM(ctx, vmOption)
	if err != nil {
		return nil, err
	}
	return v.ApplyImplicitMessage(ctx, msg)
}
