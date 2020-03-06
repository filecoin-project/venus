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
	state        state.Tree
	syscalls     SyscallsImpl
	currentEpoch abi.ChainEpoch
	pricelist    gascost.Pricelist
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

// NewVM creates a new runtime for executing messages.
// Dragons: change to take a root and the store, build the tree internally
func NewVM(actorImpls ActorImplLookup, store *storage.VMStorage, st state.Tree, syscalls SyscallsImpl) VM {
	return VM{
		context:    context.Background(),
		actorImpls: actorImpls,
		store:      store,
		state:      st,
		syscalls:   syscalls,
		// loaded during execution
		// currentEpoch: ..,
	}
}

// ApplyGenesisMessage forces the execution of a message in the vm actor.
//
// This method is intended to be used in the generation of the genesis block only.
func (vm *VM) ApplyGenesisMessage(from address.Address, to address.Address, method abi.MethodNum, value abi.TokenAmount, params interface{}, rnd crypto.RandomnessSource) (interface{}, error) {
	vm.pricelist = gascost.PricelistByEpoch(vm.currentEpoch)

	// normalize from addr
	var ok bool
	if from, ok = vm.normalizeAddress(from); !ok {
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

	// commit
	if _, err := vm.commit(); err != nil {
		return nil, err
	}

	return ret, nil
}

func (vm *VM) commit() (state.Root, error) {
	// Note: the following assumes the state commits into the store,
	//       unless the store is flushed, the state is not persisted.

	// commit the vm state
	root, err := vm.state.Commit(vm.context)
	if err != nil {
		return cid.Undef, err
	}
	// flush all blocks out of the store
	if err := vm.store.Flush(); err != nil {
		return cid.Undef, err
	}

	// TODO: update state root (issue: #3718)

	return root, nil
}

// ContextStore provides access to specs-actors adt library.
//
// This type of store is used to access some internal actor state.
func (vm *VM) ContextStore() adt.Store {
	return &contextStore{context: vm.context, store: vm.store}
}

func (vm *VM) normalizeAddress(addr address.Address) (address.Address, bool) {
	// short-circuit if the address is already an ID address
	if addr.Protocol() == address.ID {
		return addr, true
	}

	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, found, err := vm.state.GetActor(vm.context, builtin.InitActorAddr)
	if err != nil {
		panic(err)
	}
	if !found {
		runtime.Abort(exitcode.SysErrActorNotFound)
	}

	// get a view into the actor state
	var state notinit.State
	if _, err := vm.store.Get(vm.context, initActorEntry.Head.Cid, &state); err != nil {
		panic(err)
	}

	idAddr, err := state.ResolveAddress(vm.ContextStore(), addr)
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
	vm.pricelist = gascost.PricelistByEpoch(epoch)

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
		rewardMessage := makeBlockRewardMessage(blk.Miner, minerPenaltyTotal, minerGasRewardTotal, blk.TicketCount)
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
	if _, err := vm.commit(); err != nil {
		return nil, err
	}

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
	fromActor, found, err := vm.state.GetActor(vm.context, imsg.from)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("implicit message `from` field actor not found, addr: %s", imsg.from)
	}

	imsg.callSeqNumber = fromActor.CallSeqNum

	// 2. increment seq number
	fromActor.IncrementSeqNum()
	if err := vm.state.SetActor(vm.context, imsg.from, fromActor); err != nil {
		return nil, err
	}

	// 3. build context
	ctx := newInvocationContext(vm, imsg, fromActor, &gasTank, rnd)
	// 4. invoke message
	return ctx.invoke(), nil
}

// applyMessage applies the message to the current state.
func (vm *VM) applyMessage(msg *types.UnsignedMessage, onChainMsgSize int, rnd crypto.RandomnessSource) (message.Receipt, minerPenaltyFIL, gasRewardFIL) {
	// Dragons: temp until we remove legacy types
	var msgGasLimit gas.Unit = gas.Unit(msg.GasLimit)

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
	msgGasCost := vm.pricelist.OnChainMessage(onChainMsgSize)
	ok := gasTank.TryCharge(msgGasCost)
	if !ok {
		// Invalid message; insufficient gas limit to pay for the on-chain message size.
		// Note: the miner needs to pay the full msg cost, not what might have been partially consumed
		return message.Failure(exitcode.SysErrOutOfGas, gas.Zero), msgGasCost.ToTokens(msg.GasPrice), big.Zero()
	}

	// 2. load actor from global state
	if msg.From, ok = vm.normalizeAddress(msg.From); !ok {
		return message.Failure(exitcode.SysErrActorNotFound, gas.Zero), gasTank.GasConsumed().ToTokens(msg.GasPrice), big.Zero()
	}

	fromActor, found, err := vm.state.GetActor(context.Background(), msg.From)
	if err != nil {
		panic(err)
	}
	if !found {
		// Execution error; sender does not exist at time of message execution.
		return message.Failure(exitcode.SysErrActorNotFound, gas.Zero), gasTank.GasConsumed().ToTokens(msg.GasPrice), big.Zero()
	}

	// 3. make sure this is the right message order for fromActor
	if msg.CallSeqNum != fromActor.CallSeqNum {
		// Execution error; invalid seq number.
		return message.Failure(exitcode.SysErrInvalidCallSeqNum, gas.Zero), gasTank.GasConsumed().ToTokens(msg.GasPrice), big.Zero()
	}

	// 4. Check sender balance (gas + value being sent)
	gasLimitCost := msgGasLimit.ToTokens(msg.GasPrice)
	totalCost := big.Add(msg.Value, gasLimitCost)
	if fromActor.Balance.LessThan(totalCost) {
		// Execution error; sender does not have sufficient funds to pay for the gas limit.
		return message.Failure(exitcode.SysErrInsufficientFunds, gas.Zero), gasTank.GasConsumed().ToTokens(msg.GasPrice), big.Zero()
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

	if err := vm.state.SetActor(vm.context, msg.From, fromActor); err != nil {
		panic(err)
	}

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
		value:         msg.Value,
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
	ok = gasTank.TryCharge(vm.pricelist.OnChainReturnValue(&receipt))
	if !ok {
		// Insufficient gas remaining to cover the on-chain return value; proceed as in the case
		// of method execution failure.

		// Note: we are charging the caller not the miner, there is ZERO miner penalty
		return message.Failure(exitcode.SysErrOutOfGas, gasTank.GasConsumed()), big.Zero(), gasTank.GasConsumed().ToTokens(msg.GasPrice)
	}

	// 2. Success!
	return receipt, big.Zero(), gasTank.GasConsumed().ToTokens(msg.GasPrice)
}

// transfer debits money from one account and credits it to another.
//
// WARNING: this method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this method.
func (vm *VM) transfer(debitFrom address.Address, creditTo address.Address, amount abi.TokenAmount) {
	// allow only for positive amounts
	if amount.LessThan(abi.NewTokenAmount(0)) {
		panic("unreachable: negative funds transfer not allowed")
	}

	if amount.Nil() || amount.IsZero() {
		// nothing to transfer
		return
	}

	ctx := context.Background()

	// retrieve debit account
	fromActor, found, err := vm.state.GetActor(ctx, debitFrom)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: debit account not found. %s", err))
	}

	// check that account has enough balance for transfer
	if fromActor.Balance.LessThan(amount) {
		panic("unreachable: insufficient balance on debit account")
	}

	// retrieve credit account
	toActor, found, err := vm.state.GetActor(ctx, creditTo)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: credit account not found. %s", err))
	}

	// move funds
	fromActor.Balance = big.Sub(fromActor.Balance, amount)
	toActor.Balance = big.Add(toActor.Balance, amount)

	// update actors
	if err := vm.state.SetActor(ctx, debitFrom, fromActor); err != nil {
		panic(err)
	}
	if err := vm.state.SetActor(ctx, creditTo, toActor); err != nil {
		panic(err)
	}

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
// utils
//

func msgCID(msg *types.UnsignedMessage) cid.Cid {
	cid, err := msg.Cid()
	if err != nil {
		runtime.Abortf(exitcode.SysErrInternal, "Could not compute CID for message")
	}
	return cid
}

func makeBlockRewardMessage(blockMiner address.Address, penalty abi.TokenAmount, gasReward abi.TokenAmount, ticketCount int64) internalMessage {
	params := &reward.AwardBlockRewardParams{
		Miner:       blockMiner,
		Penalty:     penalty,
		GasReward:   gasReward,
		TicketCount: ticketCount,
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
	encoded, err := encoding.Encode(&adt.EmptyValue{})
	if err != nil {
		panic(fmt.Errorf("failed to encode empty cbor value: %s", err))
	}
	return internalMessage{
		from:   builtin.SystemActorAddr,
		to:     builtin.CronActorAddr,
		value:  big.Zero(),
		method: builtin.MethodsCron.EpochTick,
		params: encoded,
	}
}
