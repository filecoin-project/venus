package vmcontext

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/interpreter"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/builtin/account"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/filecoin-project/specs-actors/actors/builtin/reward"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
)

var vmlog = logging.Logger("vm.context")

// VM holds the state and executes messages over the state.
type VM struct {
	context      context.Context
	actorImpls   ActorImplLookup
	store        *storage.VMStorage
	state        state.Tree
	syscalls     SyscallsImpl
	currentHead  block.TipSetKey
	currentEpoch abi.ChainEpoch
	pricelist    gascost.Pricelist
	baseFee      abi.TokenAmount
}

// ActorImplLookup provides access to upgradeable actor code.
type ActorImplLookup interface {
	GetActorImpl(code cid.Cid) (dispatch.Dispatcher, error)
}

type minerPenaltyFIL = abi.TokenAmount

type gasRewardFIL = abi.TokenAmount

type internalMessage struct {
	from   address.Address
	to     address.Address
	value  abi.TokenAmount
	method abi.MethodNum
	params interface{}
}

// NewVM creates a new runtime for executing messages.
// Dragons: change to take a root and the store, build the tree internally
func NewVM(actorImpls ActorImplLookup, baseFee abi.TokenAmount, store *storage.VMStorage, st state.Tree, syscalls SyscallsImpl) VM {
	return VM{
		context:    context.Background(),
		actorImpls: actorImpls,
		store:      store,
		state:      st,
		syscalls:   syscalls,
		baseFee:    baseFee,
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
		runtime.Abort(exitcode.SysErrSenderInvalid)
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

func (vm *VM) revert() error {
	return vm.state.Revert()
}

func (vm *VM) snapshot() error {
	err := vm.state.Snapshot(vm.context)
	if err != nil {
		return err
	}
	return nil
}

func (vm *VM) clearSnapshot() {
	vm.state.ClearSnapshot()
}

func (vm *VM) commit() (state.Root, error) {
	// flush all blocks out of the store
	if root, err := vm.state.Flush(vm.context); err != nil {
		return cid.Undef, err
	} else {
		return root, nil
	}
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
		panic(errors.Wrapf(err, "failed to load init actor"))
	}
	if !found {
		panic(errors.Wrapf(err, "no init actor"))
	}

	// get a view into the actor state
	var state init_.State
	if _, err := vm.store.Get(vm.context, initActorEntry.Head.Cid, &state); err != nil {
		panic(err)
	}

	idAddr, found, err := state.ResolveAddress(vm.ContextStore(), addr)
	if !found {
		return address.Undef, false
	}
	if err != nil {
		panic(err)
	}
	return idAddr, true
}

func (vm *VM) stateView() SyscallsStateView {
	// The state tree's root is not committed until the end of a tipset, so we can't use the external state view
	// type for this implementation.
	// Maybe we could re-work it to use a root HAMT node rather than root CID.
	return &syscallsStateView{vm}
}

// implement VMInterpreter for VM

var _ interpreter.VMInterpreter = (*VM)(nil)

// ApplyTipSetMessages implements interpreter.VMInterpreter
func (vm *VM) ApplyTipSetMessages(blocks []interpreter.BlockMessagesInfo, head block.TipSetKey, parentEpoch abi.ChainEpoch, epoch abi.ChainEpoch, rnd crypto.RandomnessSource) ([]message.Receipt, error) {
	receipts := []message.Receipt{}

	// update current tipset
	vm.currentHead = head
	vm.currentEpoch = epoch
	vm.pricelist = gascost.PricelistByEpoch(epoch)

	for i := parentEpoch; i < epoch; i++ {
		// handle state forks    todo add by force fork
		/*		err = sm.handleStateForks(ctx, vmi.StateTree(), i, ts)
				if err != nil {
					return cid.Undef, cid.Undef, xerrors.Errorf("error handling state forks: %w", err)
				}*/

		if i > parentEpoch {
			// run cron for null rounds if any
			cronMessage := makeCronTickMessage()
			if _, err := vm.applyImplicitMessage(cronMessage, rnd); err != nil {
				return nil, err
			}
		}
		vm.SetCurrentEpoch(i + 1)
	}

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

		// Pay block reward.
		// Dragons: missing final protocol design on if/how to determine the nominal power
		rewardMessage := makeBlockRewardMessage(blk.Miner, minerPenaltyTotal, minerGasRewardTotal, 1)
		if _, err := vm.applyImplicitMessage(rewardMessage, rnd); err != nil {
			return nil, err
		}
	}

	// cron tick
	cronMessage := makeCronTickMessage()
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
func (vm *VM) applyImplicitMessage(imsg internalMessage, rnd crypto.RandomnessSource) (cbor.Marshaler, error) {
	// implicit messages gas is tracked separatly and not paid by the miner
	gasTank := NewGasTracker(gas.SystemGasLimit)

	// the execution of the implicit messages is simpler than full external/actor-actor messages
	// execution:
	// 1. load from actor
	// 2. increment seqnumber (only for accounts)
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
	originatorIsAccount := fromActor.Code.Equals(builtin.AccountActorCodeID)

	// Compute the originator address. Unlike real messages, implicit ones can be originated by
	// singleton non-account actors. Singleton addresses are reorg-proof so ok to use here.
	var originator address.Address
	if originatorIsAccount {
		// Load sender account state to obtain stable pubkey address.
		var senderState account.State
		_, err = vm.store.Get(vm.context, fromActor.Head.Cid, &senderState)
		if err != nil {
			panic(err)
		}
		originator = senderState.Address
	} else if builtin.IsBuiltinActor(fromActor.Code.Cid) {
		originator = imsg.from // Cannot resolve non-account actor to pubkey addresses.
	} else {
		panic(fmt.Sprintf("implicit message from non-account or -singleton actor code %s", fromActor.Code.Cid))
	}

	// 2. increment seq number (only for account actors).
	// The account actor distinction only makes a difference for genesis state construction via messages, where
	// some messages are sent from non-account actors (e.g. fund transfers from the reward actor).
	if originatorIsAccount {
		fromActor.IncrementSeqNum()
		if err := vm.state.SetActor(vm.context, imsg.from, fromActor); err != nil {
			return nil, err
		}
	}

	// 3. build context
	topLevel := topLevelContext{
		originatorStableAddress: originator,
		originatorCallSeq:       fromActor.CallSeqNum, // Implied CallSeqNum is that of the actor before incrementing.
		newActorAddressCount:    0,
	}

	ctx := newInvocationContext(vm, &topLevel, imsg, fromActor, &gasTank, rnd)

	// 4. invoke message
	ret, code := ctx.invoke()
	if code.IsError() {
		return nil, fmt.Errorf("invalid exit code %d during implicit message execution: from %s, to %s, method %d, value %s, params %v",
			code, imsg.from, imsg.to, imsg.method, imsg.value, imsg.params)
	}
	return ret.inner, nil
}

// applyMessage applies the message to the current state.
func (vm *VM) applyMessage(msg *types.UnsignedMessage, onChainMsgSize int, rnd crypto.RandomnessSource) (message.Receipt, minerPenaltyFIL, gasRewardFIL) {
	// This method does not actually execute the message itself,
	// but rather deals with the pre/post processing of a message.
	// (see: `invocationContext.invoke()` for the dispatch and execution)

	// initiate gas tracking
	gasTank := NewGasTracker(msg.GasLimit)

	// pre-send
	// 1. charge for message existence
	// 2. load sender actor
	// 3. check message seq number
	// 4. check if _sender_ has enough funds
	// 5. increment message seq number
	// 6. withheld maximum gas from _sender_
	// 7. snapshot state

	// 1. charge for bytes used in chain
	msgGasCost := vm.pricelist.OnChainMessage(onChainMsgSize) //todo get price list by height
	ok := gasTank.TryCharge(msgGasCost)
	if !ok {
		minerPenaltyAmount := big.Mul(vm.baseFee, big.NewInt(gasTank.gasUsed))
		// Invalid message; insufficient gas limit to pay for the on-chain message size.
		// Note: the miner needs to pay the full msg cost, not what might have been partially consumed
		return message.Failure(exitcode.SysErrOutOfGas, gas.Zero), minerPenaltyAmount, big.Zero()
	}
	minerPenaltyAmount := big.Mul(vm.baseFee, msg.GasLimit.AsBigInt())
	// 2. load actor from global state
	if msg.From, ok = vm.normalizeAddress(msg.From); !ok {
		return message.Failure(exitcode.SysErrSenderInvalid, gas.Zero), minerPenaltyAmount, big.Zero()
	}

	fromActor, found, err := vm.state.GetActor(vm.context, msg.From)
	if err != nil {
		panic(err)
	}
	if !found {
		// Execution error; sender does not exist at time of message execution.
		return message.Failure(exitcode.SysErrSenderInvalid, gas.Zero), minerPenaltyAmount, big.Zero()
	}

	if !fromActor.Code.Equals(builtin.AccountActorCodeID) {
		// Execution error; sender is not an account.
		return message.Failure(exitcode.SysErrSenderInvalid, gas.Zero), minerPenaltyAmount, big.Zero()
	}

	// 3. make sure this is the right message order for fromActor
	if msg.CallSeqNum != fromActor.CallSeqNum {
		// Execution error; invalid seq number.
		return message.Failure(exitcode.SysErrSenderStateInvalid, gas.Zero), minerPenaltyAmount, big.Zero()
	}

	// 4. Check sender balance (gas + value being sent)
	gasLimitCost := big.Mul(big.NewIntUnsigned(uint64(msg.GasLimit)), msg.GasFeeCap)
	totalCost := big.Add(msg.Value, gasLimitCost)
	if fromActor.Balance.LessThan(totalCost) {
		// Execution error; sender does not have sufficient funds to pay for the gas limit.
		return message.Failure(exitcode.SysErrSenderStateInvalid, gas.Zero), minerPenaltyAmount, big.Zero()
	}

	gasHolder := &actor.Actor{Balance: big.NewInt(0)}
	if err := vm.transferToGasHolder(msg.From, gasHolder, totalCost); err != nil {
		panic(xerrors.Errorf("failed to withdraw gas funds: %w", err))
	}
	// 5. Increment sender CallSeqNum
	fromActor.IncrementSeqNum()
	// update actor
	if err := vm.state.SetActor(vm.context, msg.From, fromActor); err != nil {
		panic(err)
	}

	// reload from actor
	// Note: balance might have changed
	fromActor, found, err = vm.state.GetActor(vm.context, msg.From)
	if err != nil {
		panic(err)
	}
	if !found {
		panic("unreachable: actor cannot possibly not exist")
	}

	// Load sender account state to obtain stable pubkey address.
	var senderState account.State
	_, err = vm.store.Get(vm.context, fromActor.Head.Cid, &senderState)
	if err != nil {
		panic(err)
	}

	// 7. snapshot state
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
		originatorStableAddress: senderState.Address,
		originatorCallSeq:       msg.CallSeqNum,
		newActorAddressCount:    0,
	}

	// 1. build internal msg
	imsg := internalMessage{
		from:   msg.From,
		to:     msg.To,
		value:  msg.Value,
		method: msg.Method,
		params: msg.Params,
	}

	// 2. build invocation context
	ctx := newInvocationContext(vm, &topLevel, imsg, fromActor, &gasTank, rnd)

	// 3. invoke
	ret, code := ctx.invoke()

	// build receipt
	receipt := message.Receipt{
		ExitCode: code,
	}
	// encode value
	receipt.ReturnValue, err = ret.ToCbor()
	if err != nil {
		// failed to encode object returned by actor
		receipt.ReturnValue = []byte{}
		receipt.ExitCode = exitcode.SysErrorIllegalActor
	}

	// post-send
	// 1. charge gas for putting the return value on the chain
	// 2. settle gas money around (unused_gas -> sender)
	// 3. success!

	// 1. charge for the space used by the return value
	// Note: the GasUsed in the message receipt does not
	ok = gasTank.TryCharge(vm.pricelist.OnChainReturnValue(len(receipt.ReturnValue)))
	if !ok {
		// Insufficient gas remaining to cover the on-chain return value; proceed as in the case
		// of method execution failure.
		receipt.ExitCode = exitcode.SysErrOutOfGas
		receipt.ReturnValue = []byte{}
	}

	// Roll back all state if the receipt's exit code is not ok.
	// This is required in addition to revert within the invocation context since top level messages can fail for
	// more reasons than internal ones. Invocation context still needs its own revert so actors can recover and
	// proceed from a nested call failure.
	if receipt.ExitCode != exitcode.Ok {
		if err := vm.revert(); err != nil {
			panic(err)
		}
	}

	// 2. settle gas money around (unused_gas -> sender)
	gasUsed := gasTank.gasUsed
	if gasUsed < 0 {
		gasUsed = 0
	}
	gasOutputs := gascost.ComputeGasOutputs(gasUsed, int64(msg.GasLimit), vm.baseFee, msg.GasFeeCap, msg.GasPremium)

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder,
		gasOutputs.BaseFeeBurn); err != nil {
		panic(xerrors.Errorf("failed to burn base fee: %w", err))
	}

	if err := vm.transferFromGasHolder(builtin.RewardActorAddr, gasHolder, gasOutputs.MinerTip); err != nil {
		panic(xerrors.Errorf("failed to give miner gas reward: %w", err))
	}

	if err := vm.transferFromGasHolder(builtin.BurntFundsActorAddr, gasHolder,
		gasOutputs.OverEstimationBurn); err != nil {
		panic(xerrors.Errorf("failed to burn overestimation fee: %w", err))
	}

	// refund unused gas
	if err := vm.transferFromGasHolder(msg.From, gasHolder, gasOutputs.Refund); err != nil {
		panic(xerrors.Errorf("failed to refund gas: %w", err))
	}

	if big.Cmp(big.NewInt(0), gasHolder.Balance) != 0 {
		panic(xerrors.Errorf("gas handling math is wrong"))
	}

	// 3. Success!
	return receipt, gasOutputs.MinerPenalty, gasOutputs.MinerTip
}

// transfer debits money from one account and credits it to another.
// avoid calling this method with a zero amount else it will perform unnecessary actor loading.
//
// WARNING: this method will panic if the the amount is negative, accounts dont exist, or have inssuficient funds.
//
// Note: this is not idiomatic, it follows the Spec expectations for this method.
func (vm *VM) transfer(debitFrom address.Address, creditTo address.Address, amount abi.TokenAmount) (*actor.Actor, *actor.Actor) {
	// allow only for positive amounts
	if amount.LessThan(abi.NewTokenAmount(0)) {
		panic("unreachable: negative funds transfer not allowed")
	}

	// retrieve debit account
	fromActor, found, err := vm.state.GetActor(vm.context, debitFrom)
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

	// debit funds
	fromActor.Balance = big.Sub(fromActor.Balance, amount)
	if err := vm.state.SetActor(vm.context, debitFrom, fromActor); err != nil {
		panic(err)
	}

	// retrieve credit account
	toActor, found, err := vm.state.GetActor(vm.context, creditTo)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: credit account not found. %s", err))
	}

	// credit funds
	toActor.Balance = big.Add(toActor.Balance, amount)
	if err := vm.state.SetActor(vm.context, creditTo, toActor); err != nil {
		panic(err)
	}
	return toActor, fromActor
}

func (vm *VM) getActorImpl(code cid.Cid) dispatch.Dispatcher {
	actorImpl, err := vm.actorImpls.GetActorImpl(code)
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
	vm.pricelist = gascost.PricelistByEpoch(current)
}

func (vm *VM) transferToGasHolder(addr address.Address, gasHolder *actor.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return xerrors.Errorf("attempted to transfer negative value to gas holder")
	}

	return vm.state.MutateActor(addr, func(a *actor.Actor) error {
		if err := deductFunds(a, amt); err != nil {
			return err
		}
		depositFunds(gasHolder, amt)
		return nil
	})
}

func (vm *VM) transferFromGasHolder(addr address.Address, gasHolder *actor.Actor, amt abi.TokenAmount) error {
	if amt.LessThan(big.NewInt(0)) {
		return xerrors.Errorf("attempted to transfer negative value from gas holder")
	}

	if amt.Equals(big.NewInt(0)) {
		return nil
	}

	return vm.state.MutateActor(addr, func(a *actor.Actor) error {
		if err := deductFunds(gasHolder, amt); err != nil {
			return err
		}
		depositFunds(a, amt)
		return nil
	})
}

func deductFunds(act *actor.Actor, amt abi.TokenAmount) error {
	if act.Balance.LessThan(amt) {
		return fmt.Errorf("not enough funds")
	}

	act.Balance = big.Sub(act.Balance, amt)
	return nil
}

func depositFunds(act *actor.Actor, amt abi.TokenAmount) {
	act.Balance = big.Add(act.Balance, amt)
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
// implement syscalls state view
//

type syscallsStateView struct {
	*VM
}

func (vm *syscallsStateView) AccountSignerAddress(ctx context.Context, accountAddr address.Address) (address.Address, error) {
	// Short-circuit when given a pubkey address.
	if accountAddr.Protocol() == address.SECP256K1 || accountAddr.Protocol() == address.BLS {
		return accountAddr, nil
	}
	actor, found, err := vm.state.GetActor(vm.context, accountAddr)
	if err != nil {
		return address.Undef, errors.Wrapf(err, "signer resolution failed to find actor %s", accountAddr)
	}
	if !found {
		return address.Undef, fmt.Errorf("signer resolution found no such actor %s", accountAddr)
	}
	var state account.State
	if _, err := vm.store.Get(vm.context, actor.Head.Cid, &state); err != nil {
		// This error is internal, shouldn't propagate as on-chain failure
		panic(fmt.Errorf("signer resolution failed to lost state for %s ", accountAddr))
	}
	return state.Address, nil
}

func (vm *syscallsStateView) MinerControlAddresses(ctx context.Context, maddr address.Address) (owner, worker address.Address, err error) {
	actor, found, err := vm.state.GetActor(vm.context, maddr)
	if err != nil {
		return address.Undef, address.Undef, errors.Wrapf(err, "miner resolution failed to find actor %s", maddr)
	}
	if !found {
		return address.Undef, address.Undef, fmt.Errorf("miner resolution found no such actor %s", maddr)
	}
	var state miner.State
	if _, err := vm.store.Get(vm.context, actor.Head.Cid, &state); err != nil {
		// This error is internal, shouldn't propagate as on-chain failure
		panic(fmt.Errorf("signer resolution failed to lost state for %s ", maddr))
	}
	minerInfo, err := state.GetInfo(vm.ContextStore())
	if err != nil {
		panic(fmt.Errorf("failed to get miner info %s ", maddr))
	}
	return minerInfo.Owner, minerInfo.Worker, nil
}

func (vm *syscallsStateView) GetNtwkVersion(ctx context.Context, ce abi.ChainEpoch) network.Version {
	//return vm.ntwkVersion(ctx, ce)
	return network.Version(0) //todo add by force fork
}

func (vm *syscallsStateView) TotalFilCircSupply(height abi.ChainEpoch, st state.Tree) (abi.TokenAmount, error) {
	return abi.TokenAmount{} //todo add by force
}

//
// utils
//

func msgCID(msg *types.UnsignedMessage) cid.Cid {
	cid, err := msg.Cid()
	if err != nil {
		panic(fmt.Sprintf("failed to compute message CID: %v; %+v", err, msg))
	}
	return cid
}

func makeBlockRewardMessage(blockMiner address.Address, penalty abi.TokenAmount, gasReward abi.TokenAmount, winCount int64) internalMessage {
	params := &reward.AwardBlockRewardParams{
		Miner:     blockMiner,
		Penalty:   penalty,
		GasReward: gasReward,
		WinCount:  winCount,
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

func makeCronTickMessage() internalMessage {
	return internalMessage{
		from:   builtin.SystemActorAddr,
		to:     builtin.CronActorAddr,
		value:  big.Zero(),
		method: builtin.MethodsCron.EpochTick,
		params: []byte{},
	}
}
