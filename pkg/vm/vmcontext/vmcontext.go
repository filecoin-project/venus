package vmcontext

import (
	"context"
	"fmt"

	builtintypes "github.com/filecoin-project/go-state-types/builtin"

	"github.com/filecoin-project/venus/pkg/constants"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"

	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	"github.com/filecoin-project/venus/pkg/state/tree"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/runtime"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	initActor "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"
	"github.com/filecoin-project/venus/venus-shared/types"
)

const MaxCallDepth = 4096

var vmlog = logging.Logger("vm.context")

// LegacyVM holds the stateView and executes messages over the stateView.
type LegacyVM struct {
	context    context.Context
	actorImpls ActorImplLookup
	bsstore    *blockstoreutil.BufferedBS
	store      cbor.IpldStore

	currentEpoch abi.ChainEpoch
	pricelist    gas.Pricelist

	debugger *VMDebugMsg // nolint
	vmOption VmOption

	baseCircSupply abi.TokenAmount

	State tree.Tree
}

func (vm *LegacyVM) ApplyImplicitMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error) {
	unsignedMsg := msg.VMMessage()

	imsg := VmMessage{
		From:   unsignedMsg.From,
		To:     unsignedMsg.To,
		Value:  unsignedMsg.Value,
		Method: unsignedMsg.Method,
		Params: unsignedMsg.Params,
	}

	return vm.applyImplicitMessage(imsg)
}

// ActorImplLookup provides access To upgradeable actor code.
type ActorImplLookup interface {
	GetActorImpl(code cid.Cid, rt runtime.Runtime) (dispatch.Dispatcher, *dispatch.ExcuteError)
}

// implement VMInterpreter for LegacyVM
var _ VMInterpreter = (*LegacyVM)(nil)

var _ Interface = (*LegacyVM)(nil)

// NewLegacyVM creates a new runtime for executing messages.
// Dragons: change To take a root and the store, build the tree internally
func NewLegacyVM(ctx context.Context, actorImpls ActorImplLookup, vmOption VmOption) (*LegacyVM, error) {
	buf := blockstoreutil.NewBufferedBstore(vmOption.Bsstore)
	cst := cbor.NewCborStore(buf)
	var st tree.Tree
	var err error
	if vmOption.PRoot == cid.Undef {
		//just for chain gen
		st, err = tree.NewState(cst, tree.StateTreeVersion1)
		if err != nil {
			panic(fmt.Errorf("create state error, should never come here"))
		}
	} else {
		st, err = tree.LoadState(context.Background(), cst, vmOption.PRoot)
		if err != nil {
			return nil, err
		}
	}

	baseCirc, err := vmOption.CircSupplyCalculator(ctx, vmOption.Epoch, st)
	if err != nil {
		return nil, err
	}

	return &LegacyVM{
		context:        ctx,
		actorImpls:     actorImpls,
		bsstore:        buf,
		store:          cst,
		State:          st,
		vmOption:       vmOption,
		baseCircSupply: baseCirc,
		pricelist:      vmOption.GasPriceSchedule.PricelistByEpoch(vmOption.Epoch),
		currentEpoch:   vmOption.Epoch,
	}, nil
}

// nolint
func (vm *LegacyVM) setDebugger() {
	vm.debugger = NewVMDebugMsg()
}

// ApplyGenesisMessage forces the execution of a message in the vm actor.
//
// This Method is intended To be used in the generation of the genesis block only.
func (vm *LegacyVM) ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}) (*Ret, error) {
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

	ret, err := vm.applyImplicitMessage(imsg)
	if err != nil {
		return ret, err
	}

	// commit
	if _, err := vm.Flush(vm.context); err != nil {
		return nil, err
	}

	return ret, nil
}

// ContextStore provides access To specs-actors adt library.
//
// This type of store is used To access some internal actor stateView.
func (vm *LegacyVM) ContextStore() adt.Store {
	return adt.WrapStore(vm.context, vm.store)
}

func (vm *LegacyVM) normalizeAddress(addr address.Address) (address.Address, bool) {
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

// applyImplicitMessage applies messages automatically generated by the vm itself.
//
// This messages do not consume client gas and must not fail.
func (vm *LegacyVM) applyImplicitMessage(imsg VmMessage) (*Ret, error) {
	// implicit messages gas is tracked separatly and not paid by the miner
	gasTank := gas.NewGasTracker(constants.BlockGasLimit * 10000)

	// the execution of the implicit messages is simpler than full external/actor-actor messages
	// execution:
	// 1. load From actor
	// 2. build new context
	// 3. invoke message

	// 1. load From actor
	fromActor, found, err := vm.State.GetActor(vm.context, imsg.From)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("implicit message `From` field actor not found, addr: %s", imsg.From)
	}
	// 2. build context
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
	cst := cbor.NewCborStore(gasBsstore)
	ctx := newInvocationContext(vm, cst, &topLevel, imsg, gasTank, vm.vmOption.Rnd, nil)

	// 3. invoke message
	ret, code := ctx.invoke()
	if code.IsError() {
		return nil, fmt.Errorf("invalid exit code %d during implicit message execution: From %s, To %s, Method %d, Value %s, Params %v",
			code, imsg.From, imsg.To, imsg.Method, imsg.Value, imsg.Params)
	}
	return &Ret{
		GasTracker: gasTank,
		OutPuts:    gas.GasOutputs{},
		Receipt: types.MessageReceipt{
			ExitCode: code,
			Return:   ret,
			GasUsed:  0,
		},
	}, nil
}

// Get the buffered blockstore associated with the LegacyVM. This includes any temporary blocks produced
// during this LegacyVM's execution.
func (vm *LegacyVM) ActorStore(ctx context.Context) adt.Store {
	return adt.WrapStore(ctx, vm.store)
}

// todo estimate gasLimit
func (vm *LegacyVM) ApplyMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error) {
	return vm.applyMessage(msg.VMMessage(), msg.ChainLength())
}

// applyMessage applies the message To the current stateView.
func (vm *LegacyVM) applyMessage(msg *types.Message, onChainMsgSize int) (*Ret, error) {
	// This Method does not actually execute the message itself,
	// but rather deals with the pre/post processing of a message.
	// (see: `invocationContext.invoke()` for the dispatch and execution)
	// initiate gas tracking
	gasTank := gas.NewGasTracker(msg.GasLimit)
	// pre-send
	// 1. charge for message existence
	// 2. load sender actor
	// 3. check message seq number
	// 4. check sender gas fee is enough
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
			Receipt:    Failure(exitcode.SysErrOutOfGas, 0),
		}, nil
	}

	minerPenaltyAmount := big.Mul(vm.vmOption.BaseFee, big.NewInt(msg.GasLimit))

	//2. load sender actor and check send whether to be an account
	fromActor, found, err := vm.State.GetActor(vm.context, msg.From)
	if err != nil {
		return nil, err
	}
	if !found {
		// Execution error; sender does not exist at time of message execution.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    Failure(exitcode.SysErrSenderInvalid, 0),
		}, nil
	}

	if !builtin.IsAccountActor(fromActor.Code) /*!fromActor.Code.Equals(builtin.AccountActorCodeID)*/ {
		// Execution error; sender is not an account.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    Failure(exitcode.SysErrSenderInvalid, 0),
		}, nil
	}

	// 3. make sure this is the right message order for fromActor
	if msg.Nonce != fromActor.Nonce {
		// Execution error; invalid seq number.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    Failure(exitcode.SysErrSenderStateInvalid, 0),
		}, nil
	}

	// 4. Check sender gas fee is enough
	gasLimitCost := big.Mul(big.NewIntUnsigned(uint64(msg.GasLimit)), msg.GasFeeCap)
	if fromActor.Balance.LessThan(gasLimitCost) {
		// Execution error; sender does not have sufficient funds To pay for the gas limit.
		gasOutputs := gas.ZeroGasOutputs()
		gasOutputs.MinerPenalty = minerPenaltyAmount
		return &Ret{
			GasTracker: gasTank,
			OutPuts:    gasOutputs,
			Receipt:    Failure(exitcode.SysErrSenderStateInvalid, 0),
		}, nil
	}

	gasHolder := &types.Actor{Balance: big.NewInt(0)}
	if err := vm.transferToGasHolder(msg.From, gasHolder, gasLimitCost); err != nil {
		return nil, fmt.Errorf("failed To withdraw gas funds: %w", err)
	}

	// 5. increment sender Nonce
	if err = vm.State.MutateActor(msg.From, func(msgFromActor *types.Actor) error {
		msgFromActor.IncrementSeqNum()
		return nil
	}); err != nil {
		return nil, err
	}

	// 7. snapshot stateView
	// Even if the message fails, the following accumulated changes will be applied:
	// - CallSeqNumber increment
	// - sender balance withheld
	err = vm.snapshot()
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}

	// 2. settle gas money around (unused_gas -> sender)
	gasUsed := gasTank.GasUsed
	if gasUsed < 0 {
		gasUsed = 0
	}

	burn, err := vm.shouldBurn(vm.context, msg, code)
	if err != nil {
		return nil, fmt.Errorf("deciding whether should burn failed: %w", err)
	}

	gasOutputs := gas.ComputeGasOutputs(gasUsed, msg.GasLimit, vm.vmOption.BaseFee, msg.GasFeeCap, msg.GasPremium, burn)

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder, gasOutputs.BaseFeeBurn); err != nil {
		return nil, fmt.Errorf("failed To burn base fee: %w", err)
	}

	if err := vm.transferFromGasHolder(reward.Address, gasHolder, gasOutputs.MinerTip); err != nil {
		return nil, fmt.Errorf("failed To give miner gas reward: %w", err)
	}

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder, gasOutputs.OverEstimationBurn); err != nil {
		return nil, fmt.Errorf("failed To burn overestimation fee: %w", err)
	}

	// refund unused gas
	if err := vm.transferFromGasHolder(msg.From, gasHolder, gasOutputs.Refund); err != nil {
		return nil, fmt.Errorf("failed To refund gas: %w", err)
	}

	if big.Cmp(big.NewInt(0), gasHolder.Balance) != 0 {
		return nil, fmt.Errorf("gas handling math is wrong")
	}

	// 3. Success!
	return &Ret{
		GasTracker: gasTank,
		OutPuts:    gasOutputs,
		Receipt: types.MessageReceipt{
			ExitCode: code,
			Return:   ret,
			GasUsed:  gasUsed,
		},
	}, nil
}

func (vm *LegacyVM) shouldBurn(ctx context.Context, msg *types.Message, errcode exitcode.ExitCode) (bool, error) {
	if vm.NetworkVersion() <= network.Version12 {
		// Check to see if we should burn funds. We avoid burning on successful
		// window post. This won't catch _indirect_ window post calls, but this
		// is the best we can get for now.
		if vm.currentEpoch > vm.vmOption.Fork.GetForkUpgrade().UpgradeClausHeight && errcode == exitcode.Ok && msg.Method == builtintypes.MethodsMiner.SubmitWindowedPoSt {
			// Ok, we've checked the _method_, but we still need to check
			// the target actor. It would be nice if we could just look at
			// the trace, but I'm not sure if that's safe?
			if toActor, _, err := vm.State.GetActor(vm.context, msg.To); err != nil {
				// If the actor wasn't found, we probably deleted it or something. Move on.
				if !errors.Is(err, types.ErrActorNotFound) {
					// Otherwise, this should never fail and something is very wrong.
					return false, fmt.Errorf("failed to lookup target actor: %w", err)
				}
			} else if builtin.IsStorageMinerActor(toActor.Code) {
				// Ok, this is a storage miner and we've processed a window post. Remove the burn.
				return false, nil
			}
		}

		return true, nil
	}

	// Any "don't burn" rules from Network v13 onwards go here, for now we always return true
	return true, nil
}

// transfer debits money From one account and credits it To another.
// avoid calling this Method with a zero amount else it will perform unnecessary actor loading.
//
// WARNING: this Method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this Method.
func (vm *LegacyVM) transfer(from address.Address, to address.Address, amount abi.TokenAmount, networkVersion network.Version) {
	var fromActor *types.Actor
	var fromID, toID address.Address
	var err error
	var found bool
	// switching the order around so that transactions for more than the balance sent to self fail
	if networkVersion >= network.Version15 {
		if amount.LessThan(big.Zero()) {
			runtime.Abortf(exitcode.SysErrForbidden, "attempt To transfer negative Value %s From %s To %s", amount, from, to)
		}

		fromID, err = vm.State.LookupID(from)
		if err != nil {
			panic(fmt.Errorf("transfer failed when resolving sender address: %s", err))
		}

		// retrieve sender account
		fromActor, found, err = vm.State.GetActor(vm.context, fromID)
		if err != nil {
			panic(err)
		}
		if !found {
			panic(fmt.Errorf("unreachable: sender account not found. %s", err))
		}

		// check that account has enough balance for transfer
		if fromActor.Balance.LessThan(amount) {
			runtime.Abortf(exitcode.SysErrInsufficientFunds, "sender %s insufficient balance %s To transfer %s To %s", amount, fromActor.Balance, from, to)
		}

		if from == to {
			vmlog.Infow("sending to same address: noop", "from/to addr", from)
			return
		}

		toID, err = vm.State.LookupID(to)
		if err != nil {
			panic(fmt.Errorf("transfer failed when resolving receiver address: %s", err))
		}

		if fromID == toID {
			vmlog.Infow("sending to same actor ID: noop", "from/to actor", fromID)
			return
		}
	} else {
		if from == to {
			return
		}

		fromID, err = vm.State.LookupID(from)
		if err != nil {
			panic(fmt.Errorf("transfer failed when resolving sender address: %s", err))
		}

		toID, err = vm.State.LookupID(to)
		if err != nil {
			panic(fmt.Errorf("transfer failed when resolving receiver address: %s", err))
		}

		if fromID == toID {
			return
		}

		if amount.LessThan(types.NewInt(0)) {
			runtime.Abortf(exitcode.SysErrForbidden, "attempt To transfer negative Value %s From %s To %s", amount, from, to)
		}

		// retrieve sender account
		fromActor, found, err = vm.State.GetActor(vm.context, fromID)
		if err != nil {
			panic(err)
		}
		if !found {
			panic(fmt.Errorf("unreachable: sender account not found. %s", err))
		}
	}

	// retrieve receiver account
	toActor, found, err := vm.State.GetActor(vm.context, toID)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: credit account not found. %s", err))
	}

	// check that account has enough balance for transfer
	if fromActor.Balance.LessThan(amount) {
		runtime.Abortf(exitcode.SysErrInsufficientFunds, "sender %s insufficient balance %s To transfer %s To %s", amount, fromActor.Balance, from, to)
	}

	// deduct funds
	fromActor.Balance = big.Sub(fromActor.Balance, amount)
	if err := vm.State.SetActor(vm.context, from, fromActor); err != nil {
		panic(err)
	}

	// deposit funds
	toActor.Balance = big.Add(toActor.Balance, amount)
	if err := vm.State.SetActor(vm.context, to, toActor); err != nil {
		panic(err)
	}
}

func (vm *LegacyVM) getActorImpl(code cid.Cid, runtime2 runtime.Runtime) dispatch.Dispatcher {
	actorImpl, err := vm.actorImpls.GetActorImpl(code, runtime2)
	if err != nil {
		runtime.Abort(exitcode.SysErrInvalidReceiver)
	}
	return actorImpl
}

//
// implement runtime.Runtime for LegacyVM
//

var _ runtime.Runtime = (*LegacyVM)(nil)

// CurrentEpoch implements runtime.Runtime.
func (vm *LegacyVM) CurrentEpoch() abi.ChainEpoch {
	return vm.currentEpoch
}

func (vm *LegacyVM) NetworkVersion() network.Version {
	return vm.vmOption.NetworkVersion
}

func (vm *LegacyVM) transferToGasHolder(addr address.Address, gasHolder *types.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return fmt.Errorf("attempted To transfer negative Value To gas holder")
	}
	return vm.State.MutateActor(addr, func(a *types.Actor) error {
		if err := deductFunds(a, amt); err != nil {
			return err
		}
		depositFunds(gasHolder, amt)
		return nil
	})
}

func (vm *LegacyVM) transferFromGasHolder(addr address.Address, gasHolder *types.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return fmt.Errorf("attempted To transfer negative Value From gas holder")
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

func (vm *LegacyVM) StateTree() tree.Tree {
	return vm.State
}

func (vm *LegacyVM) GetCircSupply(ctx context.Context) (abi.TokenAmount, error) {
	// Before v15, this was recalculated on each invocation as the state tree was mutated
	if vm.vmOption.NetworkVersion <= network.Version14 {
		return vm.vmOption.CircSupplyCalculator(ctx, vm.currentEpoch, vm.State)
	}

	return vm.baseCircSupply, nil
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

var _ rt5.Message = (*VmMessage)(nil)

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

func (vm *LegacyVM) revert() error {
	return vm.State.Revert()
}

func (vm *LegacyVM) snapshot() error {
	err := vm.State.Snapshot(vm.context)
	if err != nil {
		return err
	}
	return nil
}

func (vm *LegacyVM) clearSnapshot() {
	vm.State.ClearSnapshot()
}

//nolint
func (vm *LegacyVM) Flush(ctx context.Context) (tree.Root, error) {
	// Flush all blocks out of the store
	if root, err := vm.State.Flush(vm.context); err != nil {
		return cid.Undef, err
	} else {
		if err := blockstoreutil.CopyBlockstore(context.TODO(), vm.bsstore.Write(), vm.bsstore.Read()); err != nil {
			return cid.Undef, fmt.Errorf("copying tree: %w", err)
		}
		return root, nil
	}
}
