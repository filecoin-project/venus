package vmcontext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
	cbor "github.com/ipfs/go-ipld-cbor"
	"reflect"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/cron"
	initActor "github.com/filecoin-project/venus/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/runtime"
	"github.com/filecoin-project/venus/pkg/vm/state"
)

const MaxCallDepth = 4096

var vmlog = logging.Logger("vm.context")

// VM holds the stateView and executes messages over the stateView.
type VM struct {
	context    context.Context
	actorImpls ActorImplLookup
	bsstore    *blockstoreutil.BufferedBS
	store      cbor.IpldStore

	currentEpoch abi.ChainEpoch
	pricelist    gas.Pricelist

	vmDebug  bool // open debug or not
	debugger *VMDebugMsg
	vmOption VmOption

	State state.Tree
}

func (vm *VM) ApplyImplicitMessage(msg types.ChainMsg) (*Ret, error) {
	unsignedMsg := msg.VMMessage()

	imsg := VmMessage{
		From:   unsignedMsg.From,
		To:     unsignedMsg.To,
		Value:  unsignedMsg.Value,
		Method: unsignedMsg.Method,
		Params: unsignedMsg.Params,
	}
	vm.SetCurrentEpoch(vm.vmOption.Epoch)
	return vm.applyImplicitMessage(imsg)
}

// ActorImplLookup provides access To upgradeable actor code.
type ActorImplLookup interface {
	GetActorImpl(code cid.Cid, rt runtime.Runtime) (dispatch.Dispatcher, *dispatch.ExcuteError)
}

func VmMessageFromUnsignedMessage(msg *types.UnsignedMessage) VmMessage { //nolint
	return VmMessage{
		From:   msg.From,
		To:     msg.To,
		Value:  msg.Value,
		Method: msg.Method,
		Params: msg.Params,
	}
}

// implement VMInterpreter for VM
var _ VMInterpreter = (*VM)(nil)

// NewVM creates a new runtime for executing messages.
// Dragons: change To take a root and the store, build the tree internally
func NewVM(actorImpls ActorImplLookup, vmOption VmOption) (*VM, error) {
	buf := blockstoreutil.NewBufferedBstore(vmOption.Bsstore)
	cst := cbor.NewCborStore(buf)
	var st state.Tree
	var err error
	if vmOption.PRoot == cid.Undef {
		//just for chain gen
		st, err = state.NewState(cst, state.StateTreeVersion1)
		if err != nil {
			panic(xerrors.Errorf("create state error, should never come here"))
		}
	} else {
		st, err = state.LoadState(context.Background(), cst, vmOption.PRoot)
		if err != nil {
			return nil, err
		}
	}

	//store := storage.NewStorage(buf)
	return &VM{
		context:    context.Background(),
		actorImpls: actorImpls,
		bsstore:    buf,
		store:      cst,
		State:      st,
		vmOption:   vmOption,
		// loaded during execution
		// currentEpoch: ..,
	}, nil
}

func (vm *VM) setDebugger() {
	vm.debugger = NewVMDebugMsg()
	vm.vmDebug = true
}

// ApplyGenesisMessage forces the execution of a message in the vm actor.
//
// This Method is intended To be used in the generation of the genesis block only.
func (vm *VM) ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error) {
	// normalize From addr
	var ok bool
	if from, ok = vm.normalizeAddress(from); !ok {
		runtime.Abort(exitcode.SysErrSenderInvalid)
	}

	// build internal message
	imsg := VmMessage{
		From:   from,
		To:     to,
		Value:  value,
		Method: method,
		Params: params,
	}

	vm.SetCurrentEpoch(0)
	ret, err := vm.applyImplicitMessage(imsg)
	if err != nil {
		return ret, err
	}

	// commit
	if _, err := vm.Flush(); err != nil {
		return nil, err
	}

	return ret, nil
}

// ContextStore provides access To specs-actors adt library.
//
// This type of store is used To access some internal actor stateView.
func (vm *VM) ContextStore() adt.Store {
	return &contextStore{context: vm.context, store: vm.store}
}

func (vm *VM) normalizeAddress(addr address.Address) (address.Address, bool) {
	// short-circuit if the address is already an ID address
	if addr.Protocol() == address.ID {
		return addr, true
	}

	// resolve the target address via the InitActor, and attempt To load stateView.
	initActorEntry, found, err := vm.State.GetActor(vm.context, initActor.Address)
	if err != nil {
		panic(errors.Wrapf(err, "failed To load init actor"))
	}
	if !found {
		panic(errors.Wrapf(err, "no init actor"))
	}

	// get a view into the actor stateView
	initActorState, err := initActor.Load(adt.WrapStore(vm.context, vm.store), initActorEntry)
	if err != nil {
		panic(err)
	}

	idAddr, found, err := initActorState.ResolveAddress(addr)
	if !found {
		return address.Undef, false
	}
	if err != nil {
		panic(err)
	}
	return idAddr, true
}

// ApplyTipSetMessages implements interpreter.VMInterpreter
func (vm *VM) ApplyTipSetMessages(blocks []block.BlockMessagesInfo, ts *block.TipSet, parentEpoch, epoch abi.ChainEpoch, cb ExecCallBack) (cid.Cid, []types.MessageReceipt, error) {
	toProcessTipset := time.Now()
	var receipts []types.MessageReceipt
	pstate, _ := vm.State.Flush(vm.context)
	for i := parentEpoch; i < epoch; i++ {
		if i > parentEpoch {
			// run cron for null rounds if any
			cronMessage := makeCronTickMessage()
			ret, err := vm.applyImplicitMessage(cronMessage)
			if err != nil {
				return cid.Undef, nil, err
			}
			pstate, err = vm.Flush()
			if err != nil {
				return cid.Undef, nil, xerrors.Errorf("can not Flush vm State To db %vs", err)
			}
			if cb != nil {
				if err := cb(cid.Undef, cronMessage, ret); err != nil {
					return cid.Undef, nil, xerrors.Errorf("callback failed on cron message: %w", err)
				}
			}
		}
		// handle State forks
		// XXX: The State tree
		forkedCid, err := vm.vmOption.Fork.HandleStateForks(vm.context, pstate, i, ts)
		if err != nil {
			return cid.Undef, nil, xerrors.Errorf("hand fork error: %v", err)
		}
		vmlog.Debugf("after fork root: %s\n", forkedCid)
		if pstate != forkedCid {
			err = vm.State.At(forkedCid)
			if err != nil {
				return cid.Undef, nil, xerrors.Errorf("load fork cid error: %v", err)
			}
		}
		vm.SetCurrentEpoch(i + 1)
	}
	vmlog.Debugf("process tipset fork: %v\n", time.Now().Sub(toProcessTipset).Milliseconds())
	// create message tracker
	// Note: the same message could have been included by more than one miner
	seenMsgs := make(map[cid.Cid]struct{})

	// process messages on each block
	for index, blkInfo := range blocks {
		toProcessBlock := time.Now()
		if blkInfo.Block.Miner.Protocol() != address.ID {
			panic("precond failure: block miner address must be an IDAddress")
		}

		// initial miner penalty and gas rewards
		// Note: certain msg execution failures can cause the miner To pay for the gas
		minerPenaltyTotal := big.Zero()
		minerGasRewardTotal := big.Zero()

		// Process BLS messages From the block
		for _, m := range append(blkInfo.BlsMessages, blkInfo.SecpkMessages...) {
			// do not recompute already seen messages
			mcid := msgCID(m.VMMessage())
			if _, found := seenMsgs[mcid]; found {
				continue
			}

			// apply message
			ret := vm.applyMessage(m.VMMessage(), m.ChainLength())
			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, ret.OutPuts.MinerPenalty)
			minerGasRewardTotal = big.Add(minerGasRewardTotal, ret.OutPuts.MinerTip)
			receipts = append(receipts, ret.Receipt)
			if cb != nil {
				if err := cb(mcid, VmMessageFromUnsignedMessage(m.VMMessage()), ret); err != nil {
					return cid.Undef, nil, err
				}
			}
			// flag msg as seen
			seenMsgs[mcid] = struct{}{}

			if vm.vmDebug {
				rootCid, _ := vm.Flush()

				vm.debugger.Println("message:", mcid, "  root:", rootCid)
				msgGasOutput, _ := json.MarshalIndent(ret.OutPuts, "", "\t")
				vm.debugger.Println(string(msgGasOutput))

				valuedTraces := []*types.GasTrace{}
				for _, trace := range ret.GasTracker.ExecutionTrace.GasCharges {
					if trace.TotalGas > 0 {
						valuedTraces = append(valuedTraces, trace)
					}
				}
				tracesBytes, _ := json.MarshalIndent(valuedTraces, "", "\t")
				vm.debugger.Println(string(tracesBytes))
			}
		}

		// Pay block reward.
		// Dragons: missing final protocol design on if/how To determine the nominal power
		rewardMessage := makeBlockRewardMessage(blkInfo.Block.Miner, minerPenaltyTotal, minerGasRewardTotal, blkInfo.Block.ElectionProof.WinCount)
		ret, err := vm.applyImplicitMessage(rewardMessage)
		if err != nil {
			return cid.Undef, nil, err
		}
		if cb != nil {
			if err := cb(cid.Undef, rewardMessage, ret); err != nil {
				return cid.Undef, nil, xerrors.Errorf("callback failed on reward message: %w", err)
			}
		}

		if vm.vmDebug {
			root, _ := vm.State.Flush(context.TODO())
			vm.debugger.Println("reward: ", index, " root: ", root)
		}
		vmlog.Infof("process block %v time %v", index, time.Since(toProcessBlock).Milliseconds())
	}

	// cron tick
	toProcessCron := time.Now()
	cronMessage := makeCronTickMessage()

	ret, err := vm.applyImplicitMessage(cronMessage)
	if err != nil {
		return cid.Undef, nil, err
	}
	if cb != nil {
		if err := cb(cid.Undef, cronMessage, ret); err != nil {
			return cid.Undef, nil, xerrors.Errorf("callback failed on cron message: %w", err)
		}
	}

	vmlog.Infof("process cron: %v", time.Now().Sub(toProcessCron).Milliseconds())
	if vm.vmDebug {
		root, _ := vm.State.Flush(context.TODO())
		vm.debugger.Printfln("after cron root: %s", root)

		receipt, _ := json.MarshalIndent(receipts, "", "\t")
		vm.debugger.Println(string(receipt))
		vm.debugger.WriteToTerminal()
	}

	vmlog.Infof("process tipset %d: %v", epoch, time.Now().Sub(toProcessTipset).Milliseconds())
	// commit stateView
	root, err := vm.Flush()
	if err != nil {
		return cid.Undef, nil, err
	}
	//copy to db
	return root, receipts, nil
}

// applyImplicitMessage applies messages automatically generated by the vm itself.
//
// This messages do not consume client gas and must not fail.
func (vm *VM) applyImplicitMessage(imsg VmMessage) (*Ret, error) {
	// implicit messages gas is tracked separatly and not paid by the miner
	gasTank := gas.NewGasTracker(constants.BlockGasLimit * 10000)

	// the execution of the implicit messages is simpler than full external/actor-actor messages
	// execution:
	// 1. load From actor
	// 2. increment seqnumber (only for accounts)
	// 3. build new context
	// 4. invoke message

	// 1. load From actor
	fromActor, found, err := vm.State.GetActor(vm.context, imsg.From)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("implicit message `From` field actor not found, addr: %s", imsg.From)
	}
	//originatorIsAccount := builtin.IsAccountActor(fromActor.Code)

	// 2. increment seq number (only for account actors).
	// The account actor distinction only makes a difference for genesis stateView construction via messages, where
	// some messages are sent From non-account actors (e.g. fund transfers From the reward actor).
	//if originatorIsAccount {
	//	fromActor.IncrementSeqNum()
	//	if err := vm.State.SetActor(vm.context, imsg.From, fromActor); err != nil {
	//		return nil, err
	//	}
	//}

	// 3. build context
	topLevel := topLevelContext{
		originatorStableAddress: imsg.From,
		originatorCallSeq:       fromActor.Nonce, // Implied Nonce is that of the actor before incrementing.
		newActorAddressCount:    0,
	}

	gasBsstore := &GasChargeBlockStore{
		inner:     vm.bsstore,
		pricelist: vm.pricelist,
		gasTank:   gasTank,
	}
	//	cst.Atlas = vm.store.Atlas // associate the atlas. //todo
	cst := cbor.NewCborStore(gasBsstore)
	ctx := newInvocationContext(vm, cst, &topLevel, imsg, gasTank, vm.vmOption.Rnd, nil)

	// 4. invoke message
	ret, code := ctx.invoke()
	if code.IsError() {
		return nil, fmt.Errorf("invalid exit code %d during implicit message execution: From %s, To %s, Method %d, Value %s, Params %v",
			code, imsg.From, imsg.To, imsg.Method, imsg.Value, imsg.Params)
	}
	return &Ret{
		GasTracker: gasTank,
		OutPuts:    gas.GasOutputs{},
		Receipt: types.MessageReceipt{
			ExitCode:    code,
			ReturnValue: ret,
			GasUsed:     0,
		},
	}, nil
}

// todo estimate gasLimit
func (vm *VM) ApplyMessage(msg types.ChainMsg) *Ret {
	ret := vm.applyMessage(msg.VMMessage(), msg.ChainLength())
	return ret
}

// MutateState usage: MutateState(ctx, idAddr, func(cst cbor.IpldStore, st *ActorStateType) error {...})
func (vm *VM) MutateState(ctx context.Context, addr address.Address, fn interface{}) error {
	act, find, err := vm.State.GetActor(ctx, addr)
	if err != nil {
		return xerrors.Errorf("actor not found: %w", err)
	}

	if !find {
		return xerrors.New("actor not found")
	}

	st := reflect.New(reflect.TypeOf(fn).In(1).Elem())
	if err := vm.store.Get(ctx, act.Head, st.Interface()); err != nil {
		return xerrors.Errorf("read actor head: %w", err)
	}

	out := reflect.ValueOf(fn).Call([]reflect.Value{reflect.ValueOf(vm.store), st})
	if !out[0].IsNil() && out[0].Interface().(error) != nil {
		return out[0].Interface().(error)
	}

	head, err := vm.store.Put(ctx, st.Interface())
	if err != nil {
		return xerrors.Errorf("put new actor head: %w", err)
	}

	act.Head = head

	if err := vm.State.SetActor(ctx, addr, act); err != nil {
		return xerrors.Errorf("set actor: %w", err)
	}

	return nil
}

// applyMessage applies the message To the current stateView.
func (vm *VM) applyMessage(msg *types.UnsignedMessage, onChainMsgSize int) *Ret {
	vm.SetCurrentEpoch(vm.vmOption.Epoch)
	// This Method does not actually execute the message itself,
	// but rather deals with the pre/post processing of a message.
	// (see: `invocationContext.invoke()` for the dispatch and execution)

	// initiate gas tracking
	gasTank := gas.NewGasTracker(msg.GasLimit)

	// pre-send
	// 1. charge for message existence
	// 2. load sender actor
	// 3. check message seq number
	// 4. check if _sender_ has enough funds
	// 5. increment message seq number
	// 6. withheld maximum gas From _sender_
	// 7. snapshot stateView

	// 1. charge for bytes used in chain
	msgGasCost := vm.pricelist.OnChainMessage(onChainMsgSize) //todo get price list by height
	ok := gasTank.TryCharge(msgGasCost)
	if !ok {
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = big.Mul(vm.vmOption.BaseFee, big.NewInt(msgGasCost.Total()))
		// Invalid message; insufficient gas limit To pay for the on-chain message size.
		// Note: the miner needs To pay the full msg cost, not what might have been partially consumed
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    types.Failure(exitcode.SysErrOutOfGas, 0),
		}
	}

	minerPenaltyAmount := big.Mul(vm.vmOption.BaseFee, big.NewInt(msg.GasLimit))

	fromActor, found, err := vm.State.GetActor(vm.context, msg.From)
	if err != nil {
		panic(err)
	}
	if !found {
		// Execution error; sender does not exist at time of message execution.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    types.Failure(exitcode.SysErrSenderInvalid, 0),
		}
	}

	if !builtin.IsAccountActor(fromActor.Code) /*!fromActor.Code.Equals(builtin.AccountActorCodeID)*/ {
		// Execution error; sender is not an account.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    types.Failure(exitcode.SysErrSenderInvalid, 0),
		}
	}

	// 3. make sure this is the right message order for fromActor
	if msg.Nonce != fromActor.Nonce {
		// Execution error; invalid seq number.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    types.Failure(exitcode.SysErrSenderStateInvalid, 0),
		}
	}

	// 4. Check sender balance (gas + Value being sent)
	gasLimitCost := big.Mul(big.NewIntUnsigned(uint64(msg.GasLimit)), msg.GasFeeCap)
	//totalCost := big.Add(msg.Value, gasLimitCost) todo no Value check?
	if fromActor.Balance.LessThan(gasLimitCost) {
		// Execution error; sender does not have sufficient funds To pay for the gas limit.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    types.Failure(exitcode.SysErrSenderStateInvalid, 0),
		}
	}

	gasHolder := &types.Actor{Balance: big.NewInt(0)}
	if err := vm.transferToGasHolder(msg.From, gasHolder, gasLimitCost); err != nil {
		panic(xerrors.Errorf("failed To withdraw gas funds: %w", err))
	}

	// 5. Increment sender Nonce
	if err = vm.State.MutateActor(msg.From, func(msgFromActor *types.Actor) error {
		msgFromActor.IncrementSeqNum()
		return nil
	}); err != nil {
		panic(err)
	}

	// 7. snapshot stateView
	// Even if the message fails, the following accumulated changes will be applied:
	// - CallSeqNumber increment
	// - sender balance withheld
	err = vm.snapshot()
	if err != nil {
		panic(err)
	}
	defer vm.clearSnapshot()

	// send
	// 1. build internal message
	// 2. build invocation context
	// 3. process the msg
	topLevel := topLevelContext{
		originatorStableAddress: msg.From,
		originatorCallSeq:       msg.Nonce,
		newActorAddressCount:    0,
	}

	// 1. build internal msg
	imsg := VmMessage{
		From:   msg.From,
		To:     msg.To,
		Value:  msg.Value,
		Method: msg.Method,
		Params: msg.Params,
	}

	// 2. build invocation context
	gasBsstore := &GasChargeBlockStore{
		inner:     vm.bsstore,
		pricelist: vm.pricelist,
		gasTank:   gasTank,
	}
	cst := cbor.NewCborStore(gasBsstore)
	//	cst.Atlas = vm.store.Atlas // associate the atlas. //todo

	//Note replace from and to address here
	ctx := newInvocationContext(vm, cst, &topLevel, imsg, gasTank, vm.vmOption.Rnd, nil)

	// 3. invoke
	ret, code := ctx.invoke()
	// post-send
	// 1. charge gas for putting the return Value on the chain
	// 2. settle gas money around (unused_gas -> sender)
	// 3. success!

	// 1. charge for the space used by the return Value
	// Note: the GasUsed in the message receipt does not
	ok = gasTank.TryCharge(vm.pricelist.OnChainReturnValue(len(ret)))
	if !ok {
		// Insufficient gas remaining To cover the on-chain return Value; proceed as in the case
		// of Method execution failure.
		code = exitcode.SysErrOutOfGas
		ret = []byte{}
	}

	// Roll back all stateView if the receipt's exit code is not ok.
	// This is required in addition To revert within the invocation context since top level messages can fail for
	// more reasons than internal ones. Invocation context still needs its own revert so actors can recover and
	// proceed From a nested call failure.
	if code != exitcode.Ok {
		if err := vm.revert(); err != nil {
			panic(err)
		}
	}

	// 2. settle gas money around (unused_gas -> sender)
	gasUsed := gasTank.GasUsed
	if gasUsed < 0 {
		gasUsed = 0
	}

	burn, err := vm.shouldBurn(msg, code)
	if err != nil {
		panic(xerrors.Errorf("deciding whether should burn failed: %w", err))
	}

	gasOutputs := gas.ComputeGasOutputs(gasUsed, msg.GasLimit, vm.vmOption.BaseFee, msg.GasFeeCap, msg.GasPremium, burn)

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder, gasOutputs.BaseFeeBurn); err != nil {
		panic(xerrors.Errorf("failed To burn base fee: %w", err))
	}

	if err := vm.transferFromGasHolder(reward.Address, gasHolder, gasOutputs.MinerTip); err != nil {
		panic(xerrors.Errorf("failed To give miner gas reward: %w", err))
	}

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder, gasOutputs.OverEstimationBurn); err != nil {
		panic(xerrors.Errorf("failed To burn overestimation fee: %w", err))
	}

	// refund unused gas
	if err := vm.transferFromGasHolder(msg.From, gasHolder, gasOutputs.Refund); err != nil {
		panic(xerrors.Errorf("failed To refund gas: %w", err))
	}

	if big.Cmp(big.NewInt(0), gasHolder.Balance) != 0 {
		panic(xerrors.Errorf("gas handling math is wrong"))
	}

	// 3. Success!
	if ret == nil {
		ret = []byte{} //todo cbor marshal cant diff nil and []byte  should be fix in encoding
	}
	return &Ret{
		GasTracker: gasTank,
		OutPuts:    gasOutputs,
		Receipt: types.MessageReceipt{
			ExitCode:    code,
			ReturnValue: ret,
			GasUsed:     gasUsed,
		},
	}
}

func (vm *VM) shouldBurn(msg *types.UnsignedMessage, errcode exitcode.ExitCode) (bool, error) {
	// Check to see if we should burn funds. We avoid burning on successful
	// window post. This won't catch _indirect_ window post calls, but this
	// is the best we can get for now.

	if vm.currentEpoch > vm.vmOption.Fork.GetForkUpgrade().UpgradeClausHeight && errcode == exitcode.Ok &&
		msg.Method == miner.Methods.SubmitWindowedPoSt {
		// Ok, we've checked the _method_, but we still need to check
		// the target actor. It would be nice if we could just look at
		// the trace, but I'm not sure if that's safe?
		if toActor, _, err := vm.State.GetActor(vm.context, msg.To); err != nil {
			// If the actor wasn't found, we probably deleted it or something. Move on.
			if !xerrors.Is(err, types.ErrActorNotFound) {
				// Otherwise, this should never fail and something is very wrong.
				return false, xerrors.Errorf("failed to lookup target actor: %w", err)
			}
		} else if builtin.IsStorageMinerActor(toActor.Code) {
			// Ok, this is a storage miner and we've processed a window post. Remove the burn.
			return false, nil
		}
	}

	return true, nil
}

// transfer debits money From one account and credits it To another.
// avoid calling this Method with a zero amount else it will perform unnecessary actor loading.
//
// WARNING: this Method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this Method.
func (vm *VM) transfer(debitFrom address.Address, creditTo address.Address, amount abi.TokenAmount) {
	if amount.LessThan(big.Zero()) {
		runtime.Abortf(exitcode.SysErrForbidden, "attempt To transfer negative Value %s From %s To %s", amount, debitFrom, creditTo)
	}

	// retrieve debit account
	fromActor, found, err := vm.State.GetActor(vm.context, debitFrom)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: debit account not found. %s", err))
	}

	// retrieve credit account
	toActor, found, err := vm.State.GetActor(vm.context, creditTo)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: credit account not found. %s", err))
	}

	// check that account has enough balance for transfer
	if fromActor.Balance.LessThan(amount) {
		runtime.Abortf(exitcode.SysErrInsufficientFunds, "sender %s insufficient balance %s To transfer %s To %s", amount, fromActor.Balance, debitFrom, creditTo)
	}

	// debit funds
	fromActor.Balance = big.Sub(fromActor.Balance, amount)
	if err := vm.State.SetActor(vm.context, debitFrom, fromActor); err != nil {
		panic(err)
	}

	// credit funds
	toActor.Balance = big.Add(toActor.Balance, amount)
	if err := vm.State.SetActor(vm.context, creditTo, toActor); err != nil {
		panic(err)
	}
}

func (vm *VM) getActorImpl(code cid.Cid, runtime2 runtime.Runtime) dispatch.Dispatcher {
	actorImpl, err := vm.actorImpls.GetActorImpl(code, runtime2)
	if err != nil {
		runtime.Abort(exitcode.SysErrInvalidReceiver)
	}
	return actorImpl
}

//
// implement runtime.Runtime for VM
//

var _ runtime.Runtime = (*VM)(nil)

// CurrentEpoch implements runtime.Runtime.
func (vm *VM) CurrentEpoch() abi.ChainEpoch {
	return vm.currentEpoch
}

func (vm *VM) SetCurrentEpoch(current abi.ChainEpoch) {
	vm.currentEpoch = current
	vm.pricelist = vm.vmOption.GasPriceSchedule.PricelistByEpoch(current)
}

func (vm *VM) NtwkVersion() network.Version {
	return vm.vmOption.NtwkVersionGetter(context.TODO(), vm.currentEpoch)
}

func (vm *VM) transferToGasHolder(addr address.Address, gasHolder *types.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return xerrors.Errorf("attempted To transfer negative Value To gas holder")
	}
	return vm.State.MutateActor(addr, func(a *types.Actor) error {
		if err := deductFunds(a, amt); err != nil {
			return err
		}
		depositFunds(gasHolder, amt)
		return nil
	})
}

func (vm *VM) transferFromGasHolder(addr address.Address, gasHolder *types.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return xerrors.Errorf("attempted To transfer negative Value From gas holder")
	}

	if amt.Equals(big.NewInt(0)) {
		return nil
	}

	return vm.State.MutateActor(addr, func(a *types.Actor) error {
		if err := deductFunds(gasHolder, amt); err != nil {
			return err
		}
		depositFunds(a, amt)
		return nil
	})
}

func (vm *VM) StateTree() state.Tree {
	return vm.State
}

func deductFunds(act *types.Actor, amt abi.TokenAmount) error {
	if act.Balance.LessThan(amt) {
		return fmt.Errorf("not enough funds")
	}

	act.Balance = big.Sub(act.Balance, amt)
	return nil
}

func depositFunds(act *types.Actor, amt abi.TokenAmount) {
	act.Balance = big.Add(act.Balance, amt)
}

//
// implement runtime.MessageInfo for VmMessage
//

var _ specsruntime.Message = (*VmMessage)(nil)

type VmMessage struct { //nolint
	From   address.Address
	To     address.Address
	Value  abi.TokenAmount
	Method abi.MethodNum
	Params interface{}
}

// ValueReceived implements runtime.MessageInfo.
func (msg VmMessage) ValueReceived() abi.TokenAmount {
	return msg.Value
}

// Caller implements runtime.MessageInfo.
func (msg VmMessage) Caller() address.Address {
	return msg.From
}

// Receiver implements runtime.MessageInfo.
func (msg VmMessage) Receiver() address.Address {
	return msg.To
}

func (vm *VM) revert() error {
	return vm.State.Revert()
}

func (vm *VM) snapshot() error {
	err := vm.State.Snapshot(vm.context)
	if err != nil {
		return err
	}
	return nil
}

func (vm *VM) clearSnapshot() {
	vm.State.ClearSnapshot()
}

//nolint
func (vm *VM) Flush() (state.Root, error) {
	// Flush all blocks out of the store
	if root, err := vm.State.Flush(vm.context); err != nil {
		return cid.Undef, err
	} else {
		if err := blockstoreutil.CopyBlockstore(context.TODO(), vm.bsstore.Write(), vm.bsstore.Read()); err != nil {
			return cid.Undef, xerrors.Errorf("copying tree: %w", err)
		}
		return root, nil
	}
}

//
// utils
//

func msgCID(msg *types.UnsignedMessage) cid.Cid {
	c, err := msg.Cid()
	if err != nil {
		panic(fmt.Sprintf("failed To compute message CID: %v; %+v", err, msg))
	}
	return c
}

func makeBlockRewardMessage(blockMiner address.Address, penalty abi.TokenAmount, gasReward abi.TokenAmount, winCount int64) VmMessage {
	params := &reward.AwardBlockRewardParams{
		Miner:     blockMiner,
		Penalty:   penalty,
		GasReward: gasReward,
		WinCount:  winCount,
	}
	buf := new(bytes.Buffer)
	err := params.MarshalCBOR(buf)
	if err != nil {
		panic(fmt.Errorf("failed To encode built-in block reward. %s", err))
	}
	return VmMessage{
		From:   builtin.SystemActorAddr,
		To:     reward.Address,
		Value:  big.Zero(),
		Method: reward.Methods.AwardBlockReward,
		Params: buf.Bytes(),
	}
}

func makeCronTickMessage() VmMessage {
	return VmMessage{
		From:   builtin.SystemActorAddr,
		To:     cron.Address,
		Value:  big.Zero(),
		Method: cron.Methods.EpochTick,
		Params: []byte{},
	}
}
