package vmcontext

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	notinit "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

var vmlog = logging.Logger("vm.context")

// VM holds the state and executes messages over the state.
type VM struct {
	context      context.Context
	actorImpls   ActorImplLookup
	store        *storage.VMStorage
	state        *state.CachedTree
	currentEpoch abi.ChainEpoch
}

// ActorImplLookup provides access to upgradeable actor code.
type ActorImplLookup interface {
	GetActorImpl(code cid.Cid) (dispatch.Dispatcher, error)
}

type minerPenaltyFIL = abi.TokenAmount

type gasRewardFIL = abi.TokenAmount

type internalMessage struct {
	from          address.Address
	to            address.Address
	value         abi.TokenAmount
	method        abi.MethodNum
	params        interface{}
	callSeqNumber uint64
}

// actorStorage hides the storage methods from the actors and turns the errors into runtime panics.
type actorStorage struct {
	inner *storage.VMStorage
}

// NewVM creates a new runtime for executing messages.
func NewVM(actorImpls ActorImplLookup, store *storage.VMStorage, st state.Tree) VM {
	return VM{
		actorImpls: actorImpls,
		store:      store,
		state:      state.NewCachedTree(st),
		context:    context.Background(),
		// loaded during execution
		// currentEpoch: ..,
		// rnd: ..,
	}
}

// ApplyGenesisMessage forces the execution of a message in the vm actor.
//
// This method is intended to be used in the generation of the genesis block only.
func (vm *VM) ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}, rnd crypto.RandomnessSource) (interface{}, error) {
	// normalize from addr
	var ok bool
	if from, ok = vm.normalizeFrom(from); !ok {
		runtime.Abort(exitcode.SysErrActorNotFound)
	}

	// build internal message
	imsg := internalMessage{
		from:   from,
		to:     to,
		value:  value,
		method: method,
		params: params,
	}

	ret, err := vm.applyImplicitMessage(imsg, rnd)
	if err != nil {
		return ret, err
	}

	// commit state
	// flush all objects out
	if err := vm.store.Flush(); err != nil {
		return nil, err
	}
	// commit new actor state
	if err := vm.state.Commit(context.Background()); err != nil {
		return nil, err
	}
	// TODO: update state root (issue: #3718)
	return ret, nil
}

// ContextStore provides access to specs-actors adt library.
//
// This type of store is used to access some internal actor state.
func (vm *VM) ContextStore() adt.Store {
	return &contextStore{context: vm.context, store: vm.store}
}

func (vm *VM) normalizeFrom(from address.Address) (address.Address, bool) {
	// short-circuit if the address is already an ID address
	if from.Protocol() == address.ID {
		return from, true
	}

	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, err := vm.state.GetActor(context.Background(), builtin.InitActorAddr)
	if err != nil {
		panic(fmt.Errorf("init actor not found. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(vm.Store(), initActorEntry.Head.Cid)

	// get a view into the actor state
	var state notinit.State
	stateHandle.Readonly(&state)

	idAddr, err := state.ResolveAddress(vm.ContextStore(), from)
	if err != nil {
		return address.Undef, false
	}

	return idAddr, true
}

// implement VMInterpreter for VM

var _ interpreter.VMInterpreter = (*VM)(nil)

// ApplyTipSetMessages implements interpreter.VMInterpreter
func (vm *VM) ApplyTipSetMessages(blocks []interpreter.BlockMessagesInfo, epoch abi.ChainEpoch, rnd crypto.RandomnessSource) ([]message.Receipt, error) {
	receipts := []message.Receipt{}

	// update current epoch
	vm.currentEpoch = epoch

	// create message tracker
	// Note: the same message could have been included by more than one miner
	seenMsgs := make(map[cid.Cid]struct{})

	// process messages on each block
	for _, blk := range blocks {
		if blk.Miner.Protocol() != address.ID {
			panic("precond failure: block miner address must be an IDAddress")
		}

		// initial miner penalty and gas rewards
		// Note: certain msg execution failures can cause the miner to pay for the gas
		minerPenaltyTotal := big.Zero()
		minerGasRewardTotal := big.Zero()

		// Process BLS messages from the block
		for _, m := range blk.BLSMessages {
			// do not recompute already seen messages
			mcid := msgCID(m)
			if _, found := seenMsgs[mcid]; found {
				continue
			}

			// apply message
			receipt, minerPenaltyCurr, minerGasRewardCurr := vm.applyMessage(m, m.OnChainLen(), rnd)

			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, minerPenaltyCurr)
			minerGasRewardTotal = big.Add(minerGasRewardTotal, minerGasRewardCurr)
			receipts = append(receipts, receipt)

			// flag msg as seen
			seenMsgs[mcid] = struct{}{}
		}

		// Process SECP messages from the block
		for _, sm := range blk.SECPMessages {
			// extract unsigned message part
			m := sm.Message

			// do not recompute already seen messages
			mcid := msgCID(&m)
			if _, found := seenMsgs[mcid]; found {
				continue
			}

			// apply message
			// Note: the on-chain size for SECP messages is different
			receipt, minerPenaltyCurr, minerGasRewardCurr := vm.applyMessage(&m, sm.OnChainLen(), rnd)

			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, minerPenaltyCurr)
			minerGasRewardTotal = big.Add(minerGasRewardTotal, minerGasRewardCurr)
			receipts = append(receipts, receipt)

			// flag msg as seen
			seenMsgs[mcid] = struct{}{}
		}

		// transfer gas reward from BurntFundsActor to RewardActor
		vm.transfer(builtin.BurntFundsActorAddr, builtin.RewardActorAddr, minerGasRewardTotal)

		// Pay block reward.
		// Dragons: missing final protocol design on if/how to determine the nominal power
		rewardMessage := makeBlockRewardMessage(blk.Miner, minerPenaltyTotal, minerGasRewardTotal, big.Zero())
		if _, err := vm.applyImplicitMessage(rewardMessage, rnd); err != nil {
			return nil, err
		}
	}

	// cron tick
	firstMiner := blocks[0].Miner
	cronMessage := makeCronTickMessage(firstMiner)
	if _, err := vm.applyImplicitMessage(cronMessage, rnd); err != nil {
		return nil, err
	}

	// commit state
	// flush all objects out
	if err := vm.store.Flush(); err != nil {
		return nil, err
	}
	// commit new actor state
	if err := vm.state.Commit(context.Background()); err != nil {
		return nil, err
	}
	// TODO: update state root (issue: #3718)

	return receipts, nil
}

// applyImplicitMessage applies messages automatically generated by the vm itself.
//
// This messages do not consume client gas and must not fail.
func (vm *VM) applyImplicitMessage(imsg internalMessage, rnd crypto.RandomnessSource) (out interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case runtime.ExecutionPanic:
				p := r.(runtime.ExecutionPanic)
				err = fmt.Errorf("Invalid abort during implicit message execution. %s", p)
			default:
				// do not trap unknown panics
				debug.PrintStack()
				panic(r)
			}
		}
	}()

	// implicit messages gas is tracked separatly and not paid by the miner
	gasTank := NewGasTracker(gas.SystemGasLimit)

	// the execution of the implicit messages is simpler than full external/actor-actor messages
	// execution:
	// 1. load from actor
	// 2. increment seqnumber
	// 3. build new context
	// 4. invoke message

	// 1. load from actor
	fromActor, err := vm.state.GetActor(context.Background(), imsg.from)
	if fromActor == nil || err != nil {
		return nil, fmt.Errorf("implicit message `from` field actor not found, addr: %s", imsg.from)
	}

	imsg.callSeqNumber = fromActor.CallSeqNum

	// 2. increment seq number
	fromActor.IncrementSeqNum()

	// 3. build context
	ctx := newInvocationContext(vm, imsg, fromActor, &gasTank, rnd)

	// 4. invoke message
	return ctx.invoke(), nil
}

// applyMessage applies the message to the current state.
func (vm *VM) applyMessage(msg *types.UnsignedMessage, onChainMsgSize uint32, rnd crypto.RandomnessSource) (message.Receipt, minerPenaltyFIL, gasRewardFIL) {
	// Dragons: temp until we remove legacy types
	var msgGasLimit gas.Unit = gas.NewLegacyGas(msg.GasLimit)
	var msgGasPrice abi.TokenAmount = msg.GasPrice
	var msgValue abi.TokenAmount = msg.Value

	// This method does not actually execute the message itself,
	// but rather deals with the pre/post processing of a message.
	// (see: `invocationContext.invoke()` for the dispatch and execution)

	// initiate gas tracking
	gasTank := NewGasTracker(msgGasLimit)

	// pre-send
	// 1. charge for message existence
	// 2. load sender actor
	// 3. check message seq number
	// 4. check if _sender_ has enough funds
	// 5. increment message seq number
	// 6. withheld maximum gas from _sender_
	// 7. checkpoint state

	// 1. charge for bytes used in chain
	msgGasCost := gascost.OnChainMessage(onChainMsgSize)
	ok := gasTank.TryCharge(msgGasCost)
	if !ok {
		// Invalid message; insufficient gas limit to pay for the on-chain message size.
		// Note: the miner needs to pay the full msg cost, not what might have been partially consumed
		return message.Failure(exitcode.SysErrOutOfGas, gas.Zero), msgGasCost.ToTokens(msgGasPrice), big.Zero()
	}

	// 2. load actor from global state
	if msg.From, ok = vm.normalizeFrom(msg.From); !ok {
		return message.Failure(exitcode.SysErrActorNotFound, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice), big.Zero()
	}

	// Dragons: change this to actor, ok, error ok for found or not, err are non-recoverable IO errors and such
	fromActor, err := vm.state.GetActor(context.Background(), msg.From)
	if fromActor == nil || err != nil {
		// Execution error; sender does not exist at time of message execution.
		return message.Failure(exitcode.SysErrActorNotFound, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice), big.Zero()
	}

	// 3. make sure this is the right message order for fromActor
	if msg.CallSeqNum != fromActor.CallSeqNum {
		// Execution error; invalid seq number.
		return message.Failure(exitcode.SysErrInvalidCallSeqNum, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice), big.Zero()
	}

	// 4. Check sender balance (gas + value being sent)
	gasLimitCost := msgGasLimit.ToTokens(msgGasPrice)
	totalCost := big.Add(msgValue, gasLimitCost)
	if fromActor.Balance.LessThan(totalCost) {
		// Execution error; sender does not have sufficient funds to pay for the gas limit.
		return message.Failure(exitcode.SysErrInsufficientFunds, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice), big.Zero()
	}

	// 5. Increment sender CallSeqNum
	fromActor.IncrementSeqNum()

	// 6. Deduct gas limit funds from sender first
	// Note: this should always succeed, due to the sender balance check above
	// Note: after this point, we nede to return this funds back before exiting
	vm.transfer(msg.From, builtin.BurntFundsActorAddr, gasLimitCost)

	// 7. checkpoint state
	// Even if the message fails, the following accumulated changes will be applied:
	// - CallSeqNumber increment
	// - sender balance withheld
	// - (if did not previously exist, the new auto-created target actor)

	// 7.a. checkpoint store
	// TODO: vm.store.Checkpoint() (issue: #3718)
	// 7.b. checkpoint actor state
	// TODO: vm.state.Checkpoint() (issue: #3718)

	// send
	// 1. build internal message
	// 2. build invocation context
	// 3. process the msg

	// 1. build internal msg
	imsg := internalMessage{
		from:          msg.From,
		to:            msg.To,
		value:         msgValue,
		method:        msg.Method,
		params:        msg.Params,
		callSeqNumber: msg.CallSeqNum,
	}

	// 2. build invocation context
	ctx := newInvocationContext(vm, imsg, fromActor, &gasTank, rnd)

	// 3. process the msg (safely)
	// Note: we use panics throughout the execution to signal early termination,
	//       we need to trap them on this top level call
	safeProcess := func() (out message.Receipt) {
		// trap aborts and exitcodes
		defer func() {
			// TODO: discard "non-checkpointed" state changes (issue: #3718)

			if r := recover(); r != nil {
				switch r.(type) {
				case runtime.ExecutionPanic:
					p := r.(runtime.ExecutionPanic)
					vmlog.Warn("Abort during vm execution. %s", p)
					out = message.Failure(p.Code(), gasTank.GasConsumed())
					return
				default:
					debug.PrintStack()
					panic(r)
				}
			}
		}()

		// invoke the msg
		ret := ctx.invoke()

		// set return value
		out = message.Value(ret).WithGas(gasTank.GasConsumed())

		return
	}
	receipt := safeProcess()

	// post-send
	// 1. charge gas for putting the return value on the chain
	// 2. success!

	// 1. charge for the space used by the return value
	// Note: the GasUsed in the message receipt does not
	ok = gasTank.TryCharge(gascost.OnChainReturnValue(&receipt))
	if !ok {
		// Insufficient gas remaining to cover the on-chain return value; proceed as in the case
		// of method execution failure.

		// Note: we are charging the caller not the miner, there is ZERO miner penalty
		return message.Failure(exitcode.SysErrOutOfGas, gasTank.GasConsumed()), big.Zero(), gasTank.GasConsumed().ToTokens(msgGasPrice)
	}

	// 2. Success!
	return receipt, big.Zero(), gasTank.GasConsumed().ToTokens(msgGasPrice)
}

func (vm *VM) getMinerOwner(minerAddr address.Address) address.Address {
	minerActorEntry, err := vm.state.GetActor(context.Background(), minerAddr)
	if err != nil {
		panic(fmt.Errorf("unreachable. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(vm.Store(), minerActorEntry.Head.Cid)

	// get a view into the actor state
	var state miner.State
	stateHandle.Readonly(&state)

	return state.Info.Owner
}

func (vm *VM) settleGasBill(sender address.Address, gasTank *GasTracker, payee address.Address, gasPrice abi.TokenAmount) {
	// release unused funds that were withheld
	vm.transfer(builtin.BurntFundsActorAddr, sender, gasTank.RemainingGas().ToTokens(gasPrice))
	// pay miner for gas
	vm.transfer(builtin.BurntFundsActorAddr, payee, gasTank.GasConsumed().ToTokens(gasPrice))
}

// transfer debits money from one account and credits it to another.
//
// WARNING: this method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this method.
func (vm *VM) transfer(debitFrom address.Address, creditTo address.Address, amount abi.TokenAmount) {
	// allow only for positive amounts
	// Dragons: can we please please add some happy functions and a proper type
	if amount.LessThan(abi.NewTokenAmount(0)) {
		panic("unreachable: negative funds transfer not allowed")
	}

	if amount.Nil() || amount.IsZero() {
		// nothing to transfer
		return
	}

	ctx := context.Background()

	// retrieve debit account
	fromActor, err := vm.state.GetActor(ctx, debitFrom)
	if err != nil {
		panic(fmt.Errorf("unreachable: debit account not found. %s", err))
	}

	// check that account has enough balance for transfer
	if fromActor.Balance.LessThan(amount) {
		panic("unreachable: insufficient balance on debit account")
	}

	// retrieve credit account
	toActor, err := vm.state.GetActor(ctx, creditTo)
	if err != nil {
		panic(fmt.Errorf("unreachable: credit account not found. %s", err))
	}

	// move funds
	fromActor.Balance = big.Sub(fromActor.Balance, amount)
	toActor.Balance = big.Add(toActor.Balance, amount)

	return
}

func (vm *VM) getActorImpl(code cid.Cid) dispatch.Dispatcher {
	actorImpl, err := vm.actorImpls.GetActorImpl(code)
	if err != nil {
		runtime.Abort(exitcode.SysErrActorCodeNotFound)
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

// Store implements runtime.Runtime.
func (vm *VM) Store() specsruntime.Store {
	return actorStorage{inner: vm.store}
}

//
// implement runtime.MessageInfo for internalMessage
//

var _ specsruntime.Message = (*internalMessage)(nil)

// ValueReceived implements runtime.MessageInfo.
func (msg internalMessage) ValueReceived() abi.TokenAmount {
	return msg.value
}

// Caller implements runtime.MessageInfo.
func (msg internalMessage) Caller() address.Address {
	return msg.from
}

// Receiver implements runtime.MessageInfo.
func (msg internalMessage) Receiver() address.Address {
	return msg.to
}

//
// implement runtime.Store for actorStorage
//

var _ specsruntime.Store = (*actorStorage)(nil)

func (s actorStorage) Put(obj specsruntime.CBORMarshaler) cid.Cid {
	cid, err := s.inner.Put(obj)
	if err != nil {
		panic(fmt.Errorf("could not put object in store. %s", err))
	}
	return cid
}

func (s actorStorage) Get(cid cid.Cid, obj specsruntime.CBORUnmarshaler) bool {
	err := s.inner.Get(cid, obj)
	if err == storage.ErrNotFound {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("could not get obj from store. %s", err))
	}
	return true
}

//
// utils
//

func msgCID(msg *types.UnsignedMessage) cid.Cid {
	cid, err := msg.Cid()
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not compute CID for message")
	}
	return cid
}

func makeBlockRewardMessage(blockMiner address.Address, penalty abi.TokenAmount, gasReward abi.TokenAmount, nominalPower abi.StoragePower) internalMessage {
	params := reward.AwardBlockRewardParams{
		Miner:        blockMiner,
		Penalty:      penalty,
		GasReward:    gasReward,
		NominalPower: nominalPower,
	}
	encoded, err := encoding.Encode(params)
	if err != nil {
		panic(fmt.Errorf("failed to encode built-in block reward. %s", err))
	}
	return internalMessage{
		from:   builtin.SystemActorAddr,
		to:     builtin.RewardActorAddr,
		value:  big.Zero(),
		method: builtin.MethodsReward.AwardBlockReward,
		params: encoded,
	}
}

func makeCronTickMessage(blockMiner address.Address) internalMessage {
	return internalMessage{
		from:   builtin.SystemActorAddr,
		to:     builtin.CronActorAddr,
		value:  big.Zero(),
		method: builtin.MethodsCron.EpochTick,
		params: []byte{},
	}
}
