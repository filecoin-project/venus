package vmcontext

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/cron"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/reward"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/storagemining"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
)

var vmLog = logging.Logger("vm.context")

// VM holds the state and executes messages over the state.
type VM struct {
	rnd          RandomnessSource
	actorImpls   ActorImplLookup
	store        *storage.VMStorage
	state        *state.CachedTree
	currentEpoch abi.ChainEpoch
	context      context.Context
}

// RandomnessSource provides randomness to actors.
type RandomnessSource interface {
	Randomness(epoch abi.ChainEpoch) abi.RandomnessSeed
}

// ActorImplLookup provides access to upgradeable actor code.
type ActorImplLookup interface {
	GetActorImpl(code cid.Cid) (dispatch.Dispatcher, error)
}

// MinerPenaltyFIL is just a alias for FIL used to penalize the miner
type MinerPenaltyFIL = abi.TokenAmount

type internalMessage struct {
	miner         address.Address
	from          address.Address
	to            address.Address
	value         abi.TokenAmount
	method        types.MethodID
	params        []byte
	callSeqNumber types.Uint64
}

// actorStorage hides the storage methods from the actors and turns the errors into runtime panics.
type actorStorage struct {
	inner *storage.VMStorage
}

// NewVM creates a new runtime for executing messages.
func NewVM(rnd RandomnessSource, actorImpls ActorImplLookup, store *storage.VMStorage, st state.Tree) VM {
	return VM{
		rnd:          rnd,
		actorImpls:   actorImpls,
		store:        store,
		state:        state.NewCachedTree(st),
		currentEpoch: 0,
		context:      context.Background(),
	}
}

// ApplyGenesisMessage forces the execution of a message in the vm actor.
//
// This method is intended to be used in the generation of the genesis block only.
func (vm *VM) ApplyGenesisMessage(from address.Address, to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) (interface{}, error) {
	// get the params
	// try to use the bytes directly
	encodedParams, ok := params.([]byte)
	if !ok && params != nil {
		// we got an object, encode it
		var err error
		encodedParams, err = encoding.Encode(params)
		if err != nil {
			return nil, err
		}
	}

	// normalize from addr
	from = vm.normalizeFrom(from)

	// build internal message
	imsg := internalMessage{
		miner:  address.Undef,
		from:   from,
		to:     to,
		value:  value,
		method: method,
		params: encodedParams,
	}

	ret, err := vm.applyImplicitMessage(imsg)
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

func (vm *VM) normalizeFrom(from address.Address) address.Address {
	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, err := vm.state.GetActor(context.Background(), vmaddr.InitAddress)
	if err != nil {
		panic(fmt.Errorf("init actor not found. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(vm.Storage(), initActorEntry.Head.Cid)

	// get a view into the actor state
	initView := initactor.NewView(stateHandle, vm.Storage())

	// lookup the ActorID based on the address
	targetIDAddr, ok := initView.GetIDAddressByAddress(from)
	if !ok {
		runtime.Abort(exitcode.SysErrActorNotFound)
	}

	return targetIDAddr
}

// implement VMInterpreter for VM

var _ interpreter.VMInterpreter = (*VM)(nil)

// ApplyTipSetMessages implements interpreter.VMInterpreter
func (vm *VM) ApplyTipSetMessages(blocks []interpreter.BlockMessagesInfo, epoch abi.ChainEpoch) ([]message.Receipt, error) {
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
		// Process block miner's Election PoSt.
		epostMessage := makeEpostMessage(blk.Miner)
		if _, err := vm.applyImplicitMessage(epostMessage); err != nil {
			return nil, err
		}

		// initial miner penalty
		// Note: certain msg execution failures can cause the miner to pay for the gas
		minerPenaltyTotal := big.Zero()

		// Process BLS messages from the block
		for _, m := range blk.BLSMessages {
			// do not recompute already seen messages
			mcid := msgCID(m)
			if _, found := seenMsgs[mcid]; found {
				continue
			}

			// apply message
			receipt, minerPenaltyCurr := vm.applyMessage(m, m.OnChainLen(), blk.Miner)

			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, minerPenaltyCurr)
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
			receipt, minerPenaltyCurr := vm.applyMessage(&m, sm.OnChainLen(), blk.Miner)

			// accumulate result
			minerPenaltyTotal = big.Add(minerPenaltyTotal, minerPenaltyCurr)
			receipts = append(receipts, receipt)

			// flag msg as seen
			seenMsgs[mcid] = struct{}{}
		}

		// Pay block reward.
		rewardMessage := makeBlockRewardMessage(blk.Miner, minerPenaltyTotal)
		if _, err := vm.applyImplicitMessage(rewardMessage); err != nil {
			return nil, err
		}
	}

	// cron tick
	firstMiner := blocks[0].Miner
	cronMessage := makeCronTickMessage(firstMiner)
	if _, err := vm.applyImplicitMessage(cronMessage); err != nil {
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
// This messages do not consume client gas are do not fail.
func (vm *VM) applyImplicitMessage(imsg internalMessage) (out interface{}, err error) {
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
	gasTank := gas.NewTracker(gas.SystemGasLimit)

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
	ctx := newInvocationContext(vm, imsg, fromActor, &gasTank)

	// 4. invoke message
	return ctx.invoke(), nil
}

// applyMessage applies the message to the current state.
func (vm *VM) applyMessage(msg *types.UnsignedMessage, onChainMsgSize uint32, miner address.Address) (message.Receipt, MinerPenaltyFIL) {
	// Dragons: temp until we remove legacy types
	var msgGasLimit gas.Unit = gas.NewLegacyGas(msg.GasLimit)
	var msgGasPrice abi.TokenAmount = abi.NewTokenAmount(int64(msg.GasLimit))
	var msgValue abi.TokenAmount = abi.NewTokenAmount(msg.Value.AsBigInt().Int64())

	// This method does not actually execute the message itself,
	// but rather deals with the pre/post processing of a message.
	// (see: `invocationContext.invoke()` for the dispatch and execution)

	// initiate gas tracking
	gasTank := gas.NewTracker(msgGasLimit)

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
		return message.Failure(exitcode.SysErrOutOfGas, gas.Zero), msgGasCost.ToTokens(msgGasPrice)
	}

	// 2. load actor from global state
	msg.From = vm.normalizeFrom(msg.From)
	// Dragons: change this to actor, ok, error ok for found or not, err are non-recoverable IO errors and such
	fromActor, err := vm.state.GetActor(context.Background(), msg.From)
	if fromActor == nil || err != nil {
		// Execution error; sender does not exist at time of message execution.
		return message.Failure(exitcode.SysErrActorNotFound, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice)
	}

	// 3. make sure this is the right message order for fromActor
	if msg.CallSeqNum != fromActor.CallSeqNum {
		// Execution error; invalid seq number.
		return message.Failure(exitcode.SysErrInvalidCallSeqNum, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice)
	}

	// 4. Check sender balance (gas + value being sent)
	gasLimitCost := msgGasLimit.ToTokens(msgGasPrice)
	totalCost := big.Add(msgValue, gasLimitCost)
	if fromActor.Balance.LessThan(totalCost) {
		// Execution error; sender does not have sufficient funds to pay for the gas limit.
		return message.Failure(exitcode.SysErrInsufficientFunds, gas.Zero), gasTank.GasConsumed().ToTokens(msgGasPrice)
	}

	// 5. Increment sender CallSeqNum
	fromActor.IncrementSeqNum()

	// 6. Deduct gas limit funds from sender first
	// Note: this should always succeed, due to the sender balance check above
	// Note: after this point, we nede to return this funds back before exiting
	vm.transfer(msg.From, vmaddr.BurntFundsAddress, gasLimitCost)

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
		miner:         miner,
		from:          msg.From,
		to:            msg.To,
		value:         msgValue,
		method:        msg.Method,
		params:        msg.Params,
		callSeqNumber: msg.CallSeqNum,
	}

	// 2. build invocation context
	ctx := newInvocationContext(vm, imsg, fromActor, &gasTank)

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
					vmLog.Warn("Abort during vm execution. %s", p)
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

		// get miner owner
		minerOwner := vm.getMinerOwner(miner)

		// have the sender pay the outstanding bill and get the rest of the money back
		vm.settleGasBill(msg.From, &gasTank, minerOwner, msgGasPrice)

		// Note: we are charging the caller not the miner, there is ZERO miner penalty
		return message.Failure(exitcode.SysErrOutOfGas, gasTank.GasConsumed()), big.Zero()
	}

	// 2. Success!
	return receipt, big.Zero()
}

func (vm *VM) getMinerOwner(minerAddr address.Address) address.Address {
	minerActorEntry, err := vm.state.GetActor(context.Background(), minerAddr)
	if err != nil {
		panic(fmt.Errorf("unreachable. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(vm.Storage(), minerActorEntry.Head.Cid)

	// get a view into the actor state
	minerView := miner.NewView(stateHandle, vm.Storage())

	return minerView.Owner()
}

func (vm *VM) settleGasBill(sender address.Address, gasTank *gas.Tracker, payee address.Address, gasPrice abi.TokenAmount) {
	// release unused funds that were withheld
	vm.transfer(vmaddr.BurntFundsAddress, sender, gasTank.RemainingGas().ToTokens(gasPrice))
	// pay miner for gas
	vm.transfer(vmaddr.BurntFundsAddress, payee, gasTank.GasConsumed().ToTokens(gasPrice))
}

// transfer debits money from one account and credits it to another.
//
// WARNING: this method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this method.
func (vm *VM) transfer(debitFrom address.Address, creditTo address.Address, amount abi.TokenAmount) {
	// allow only for positive amounts
	// Dragons: can we please please add some happy functions and a propper type
	if amount.LessThan(abi.NewTokenAmount(0)) {
		panic("unreachable: negative funds transfer not allowed")
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

// Randomness implements runtime.Runtime.
func (vm *VM) Randomness(epoch abi.ChainEpoch) abi.RandomnessSeed {
	return vm.rnd.Randomness(epoch)
}

// Storage implements runtime.Runtime.
func (vm *VM) Storage() runtime.Storage {
	return actorStorage{inner: vm.store}
}

//
// implement runtime.MessageInfo for internalMessage
//

var _ specsruntime.Message = (*internalMessage)(nil)

// BlockMiner implements runtime.MessageInfo.
func (msg internalMessage) BlockMiner() address.Address {
	return msg.miner
}

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
// implement runtime.Storage for actorStorage
//

var _ runtime.Storage = (*actorStorage)(nil)

func (s actorStorage) Put(obj interface{}) cid.Cid {
	cid, err := s.inner.Put(obj)
	if err != nil {
		panic(fmt.Errorf("could not put object in store. %s", err))
	}
	return cid
}

func (s actorStorage) CidOf(obj interface{}) cid.Cid {
	cid, err := s.inner.CidOf(obj)
	if err != nil {
		panic(fmt.Errorf("could not calculate cid for obj. %s", err))
	}
	return cid
}

func (s actorStorage) Get(cid cid.Cid, obj interface{}) bool {
	err := s.inner.Get(cid, obj)
	if err == storage.ErrNotFound {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("could not get obj from store. %s", err))
	}
	return true
}

func (s actorStorage) GetRaw(cid cid.Cid) ([]byte, bool) {
	raw, err := s.inner.GetRaw(cid)
	if err == storage.ErrNotFound {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("could not get obj from store. %s", err))
	}
	return raw, true
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

func makeEpostMessage(blockMiner address.Address) internalMessage {
	return internalMessage{
		miner:  blockMiner,
		from:   vmaddr.SystemAddress,
		to:     blockMiner,
		value:  big.Zero(),
		method: storagemining.ProcessVerifiedElectionPoStMethodID,
		params: []byte{},
	}
}

func makeBlockRewardMessage(blockMiner address.Address, penalty abi.TokenAmount) internalMessage {
	params := []interface{}{blockMiner, penalty}
	encoded, err := encoding.Encode(params)
	if err != nil {
		panic(fmt.Errorf("failed to encode built-in block reward. %s", err))
	}
	return internalMessage{
		miner:  blockMiner,
		from:   vmaddr.SystemAddress,
		to:     vmaddr.RewardAddress,
		value:  big.Zero(),
		method: reward.AwardBlockRewardMethodID,
		params: encoded,
	}
}

func makeCronTickMessage(blockMiner address.Address) internalMessage {
	return internalMessage{
		miner:  blockMiner,
		from:   vmaddr.SystemAddress,
		to:     vmaddr.CronAddress,
		value:  big.Zero(),
		method: cron.EpochTickMethodID,
		params: []byte{},
	}
}
