package statemanger

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/fvm"

	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	blockstoreutil "github.com/filecoin-project/venus/venus-shared/blockstore"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/vm"
)

// Call applies the given message to the given tipset's parent state, at the epoch following the
// tipset's parent. In the presence of null blocks, the height at which the message is invoked may
// be less than the specified tipset.
func (s *Stmgr) Call(ctx context.Context, msg *types.Message, ts *types.TipSet) (*types.InvocResult, error) {
	// Copy the message as we modify it below.
	msgCopy := *msg
	msg = &msgCopy

	if msg.GasLimit == 0 {
		msg.GasLimit = constants.BlockGasLimit
	}
	if msg.GasFeeCap == types.EmptyInt {
		msg.GasFeeCap = types.NewInt(0)
	}
	if msg.GasPremium == types.EmptyInt {
		msg.GasPremium = types.NewInt(0)
	}
	if msg.Value == types.EmptyInt {
		msg.Value = types.NewInt(0)
	}

	return s.callInternal(ctx, msg, nil, ts, cid.Undef, s.GetNetworkVersion, false, false)
}

// CallWithGas calculates the state for a given tipset, and then applies the given message on top of that state.
func (s *Stmgr) CallWithGas(ctx context.Context, msg *types.Message, priorMsgs []types.ChainMsg, ts *types.TipSet, applyTSMessages bool) (*types.InvocResult, error) {
	return s.callInternal(ctx, msg, priorMsgs, ts, cid.Undef, s.GetNetworkVersion, true, applyTSMessages)
}

// CallAtStateAndVersion allows you to specify a message to execute on the given stateCid and network version.
// This should mostly be used for gas modelling on a migrated state.
// Tipset here is not needed because stateCid and network version fully describe execution we want. The internal function
// will get the heaviest tipset for use for things like basefee, which we don't really care about here.
func (s *Stmgr) CallAtStateAndVersion(ctx context.Context, msg *types.Message, stateCid cid.Cid, v network.Version) (*types.InvocResult, error) {
	nvGetter := func(context.Context, abi.ChainEpoch) network.Version {
		return v
	}

	return s.callInternal(ctx, msg, nil, nil, stateCid, nvGetter, true, false)
}

//   - If no tipset is specified, the first tipset without an expensive migration or one in its parent is used.
//   - If executing a message at a given tipset or its parent would trigger an expensive migration, the call will
//     fail with ErrExpensiveFork.
func (s *Stmgr) callInternal(ctx context.Context, msg *types.Message, priorMsgs []types.ChainMsg, ts *types.TipSet, stateCid cid.Cid, nvGetter chain.NetworkVersionGetter, checkGas, applyTSMessages bool) (*types.InvocResult, error) {
	ctx, span := trace.StartSpan(ctx, "statemanager.callInternal")
	defer span.End()

	// Copy the message as we'll be modifying the nonce.
	msgCopy := *msg
	msg = &msgCopy

	var err error
	var pts *types.TipSet
	if ts == nil {
		ts = s.cs.GetHead()

		// Search back till we find a height with no fork, or we reach the beginning.
		// We need the _previous_ height to have no fork, because we'll
		// run the fork logic in `sm.TipSetState`. We need the _current_
		// height to have no fork, because we'll run it inside this
		// function before executing the given message.
		for ts.Height() > 0 {
			pts, err = s.cs.GetTipSet(ctx, ts.Parents())
			if err != nil {
				return nil, fmt.Errorf("failed to find a non-forking epoch: %w", err)
			}
			// Checks for expensive forks from the parents to the tipset, including nil tipsets
			if !s.fork.HasExpensiveForkBetween(pts.Height(), ts.Height()+1) {
				break
			}

			ts = pts
		}
	} else if ts.Height() > 0 {
		pts, err = s.cs.GetTipSet(ctx, ts.Parents())
		if err != nil {
			return nil, fmt.Errorf("failed to find a non-forking epoch: %w", err)
		}
		if s.fork.HasExpensiveForkBetween(pts.Height(), ts.Height()+1) {
			return nil, fork.ErrExpensiveFork
		}
	}

	// Unless executing on a specific state cid, apply all the messages from the current tipset
	// first. Unfortunately, we can't just execute the tipset, because that will run cron. We
	// don't want to apply miner messages after cron runs in a given epoch.
	if stateCid == cid.Undef {
		stateCid = ts.ParentState()
	}
	tsMsgs, err := s.ms.MessagesForTipset(ts)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup messages for parent tipset: %w", err)
	}
	if applyTSMessages {
		priorMsgs = append(tsMsgs, priorMsgs...)
	} else {
		var filteredTSMsgs []types.ChainMsg
		for _, tsMsg := range tsMsgs {
			//TODO we should technically be normalizing the filecoin address of from when we compare here
			if tsMsg.VMMessage().From == msg.VMMessage().From {
				filteredTSMsgs = append(filteredTSMsgs, tsMsg)
			}
		}
		priorMsgs = append(filteredTSMsgs, priorMsgs...)
	}

	// Technically, the tipset we're passing in here should be ts+1, but that may not exist.
	stateCid, err = s.fork.HandleStateForks(ctx, stateCid, ts.Height(), ts)
	if err != nil {
		return nil, fmt.Errorf("failed to handle fork: %w", err)
	}

	if span.IsRecordingEvents() {
		span.AddAttributes(
			trace.Int64Attribute("gas_limit", msg.GasLimit),
			trace.StringAttribute("gas_feecap", msg.GasFeeCap.String()),
			trace.StringAttribute("value", msg.Value.String()),
		)
	}

	random := chain.NewChainRandomnessSource(s.cs, ts.Key(), s.beacon, s.GetNetworkVersion)
	buffStore := blockstoreutil.NewTieredBstore(s.cs.Blockstore(), blockstoreutil.NewTemporarySync())
	vmopt := vm.VmOption{
		CircSupplyCalculator: func(ctx context.Context, epoch abi.ChainEpoch, tree tree.Tree) (abi.TokenAmount, error) {
			cs, err := s.cs.GetCirculatingSupplyDetailed(ctx, epoch, tree)
			if err != nil {
				return abi.TokenAmount{}, err
			}
			return cs.FilCirculating, nil
		},
		PRoot:               stateCid,
		Epoch:               ts.Height(),
		Timestamp:           ts.MinTimestamp(),
		Rnd:                 random,
		Bsstore:             buffStore,
		SysCallsImpl:        s.syscallsImpl,
		GasPriceSchedule:    s.gasSchedule,
		NetworkVersion:      nvGetter(ctx, ts.Height()),
		BaseFee:             ts.Blocks()[0].ParentBaseFee,
		Fork:                s.fork,
		LookbackStateGetter: vmcontext.LookbackStateGetterForTipset(ctx, s.cs, s.fork, ts),
		TipSetGetter:        vmcontext.TipSetGetterForTipset(s.cs.GetTipSetByHeight, ts),
		Tracing:             true,
		ActorDebugging:      s.actorDebugging,
	}
	vmi, err := fvm.NewVM(ctx, vmopt)
	if err != nil {
		return nil, fmt.Errorf("failed to set up vm: %w", err)
	}
	for i, m := range priorMsgs {
		_, err = vmi.ApplyMessage(ctx, m)
		if err != nil {
			return nil, fmt.Errorf("applying prior message (%d, %s): %w", i, m.Cid(), err)
		}
	}

	// We flush to get the VM's view of the state tree after applying the above messages
	// This is needed to get the correct nonce from the actor state to match the VM
	stateCid, err = vmi.Flush(ctx)
	if err != nil {
		return nil, fmt.Errorf("flushing vm: %w", err)
	}

	st, err := tree.LoadState(ctx, cbor.NewCborStore(buffStore), stateCid)
	if err != nil {
		return nil, fmt.Errorf("loading state: %v", err)
	}

	fromActor, found, err := st.GetActor(ctx, msg.From)
	if err != nil || !found {
		return nil, fmt.Errorf("call raw get actor: %s", err)
	}

	msg.Nonce = fromActor.Nonce

	// If the fee cap is set to zero, make gas free.
	if msg.GasFeeCap.NilOrZero() {
		// Now estimate with a new VM with no base fee.
		vmopt.BaseFee = big.Zero()
		vmopt.PRoot = stateCid

		vmi, err = fvm.NewVM(ctx, vmopt)
		if err != nil {
			return nil, fmt.Errorf("failed to set up estimation vm: %w", err)
		}
	}

	var ret *vm.Ret
	var gasInfo types.MsgGasCost
	if checkGas {
		fromKey, err := s.ResolveToDeterministicAddress(ctx, msg.From, ts)
		if err != nil {
			return nil, fmt.Errorf("could not resolve key: %w", err)
		}

		var msgApply types.ChainMsg

		switch fromKey.Protocol() {
		case address.BLS:
			msgApply = msg
		case address.SECP256K1:
			msgApply = &types.SignedMessage{
				Message: *msg,
				Signature: crypto.Signature{
					Type: crypto.SigTypeSecp256k1,
					Data: make([]byte, 65),
				},
			}
		case address.Delegated:
			msgApply = &types.SignedMessage{
				Message: *msg,
				Signature: crypto.Signature{
					Type: crypto.SigTypeDelegated,
					Data: make([]byte, 65),
				},
			}
		}

		ret, err = vmi.ApplyMessage(ctx, msgApply)
		if err != nil {
			return nil, fmt.Errorf("gas estimation failed: %w", err)
		}
		gasInfo = MakeMsgGasCost(msg, ret)
	} else {
		ret, err = vmi.ApplyImplicitMessage(ctx, msg)
		if err != nil && ret == nil {
			return nil, fmt.Errorf("apply message failed: %w", err)
		}
	}

	var errs string
	if ret.ActorErr != nil {
		errs = ret.ActorErr.Error()
	}

	return &types.InvocResult{
		MsgCid:         msg.Cid(),
		Msg:            msg,
		MsgRct:         &ret.Receipt,
		GasCost:        gasInfo,
		ExecutionTrace: ret.GasTracker.ExecutionTrace,
		Error:          errs,
		Duration:       ret.Duration,
	}, err
}
