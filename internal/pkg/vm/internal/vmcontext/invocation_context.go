package vmcontext

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

var gasOnActorExec = gas.NewGasCharge("OnActorExec", 0, 0)

// Context for a top-level invocation sequence.
type topLevelContext struct {
	originatorStableAddress address.Address // Stable (public key) address of the top-level message sender.
	originatorCallSeq       uint64          // Call sequence number of the top-level message.
	newActorAddressCount    uint64          // Count of calls to NewActorAddress (mutable).
}

// Context for an individual message invocation, including inter-actor sends.
type invocationContext struct {
	rt                *VM
	topLevel          *topLevelContext
	msg               internalMessage // The message being processed
	fromActor         *actor.Actor    // The immediate calling actor
	gasTank           *GasTracker
	randSource        crypto.RandomnessSource
	isCallerValidated bool
	allowSideEffects  bool
	toActor           *actor.Actor // The receiving actor
	stateHandle       internalActorStateHandle
}

type internalActorStateHandle interface {
	specsruntime.StateHandle
	Validate(func(interface{}) cid.Cid)
}

func newInvocationContext(rt *VM, topLevel *topLevelContext, msg internalMessage, fromActor *actor.Actor, gasTank *GasTracker, randSource crypto.RandomnessSource) invocationContext {
	// Note: the toActor and stateHandle are loaded during the `invoke()`
	return invocationContext{
		rt:                rt,
		topLevel:          topLevel,
		msg:               msg,
		fromActor:         fromActor,
		gasTank:           gasTank,
		randSource:        randSource,
		isCallerValidated: false,
		allowSideEffects:  true,
		toActor:           nil,
		stateHandle:       nil,
	}
}

type stateHandleContext invocationContext

func (shc *stateHandleContext) AllowSideEffects(allow bool) {
	shc.allowSideEffects = allow
}

func (shc *stateHandleContext) Create(obj cbor.Marshaler) cid.Cid {
	actr := shc.loadActor()
	if actr.Head.Defined() {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "failed to construct actor stateView: already initialized")
	}
	c := shc.store().StorePut(obj)
	actr.Head = enccid.NewCid(c)
	shc.storeActor(actr)
	return c
}

func (shc *stateHandleContext) Load(obj cbor.Unmarshaler) cid.Cid {
	// The actor must be loaded from store every time since the stateView may have changed via a different stateView handle
	// (e.g. in a recursive call).
	actr := shc.loadActor()
	c := actr.Head.Cid
	if !c.Defined() {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "failed to load undefined stateView, must construct first")
	}
	found := shc.store().StoreGet(c, obj)
	if !found {
		panic(fmt.Errorf("failed to load stateView for actor %s, CID %s", shc.msg.to, c))
	}
	return c
}

func (shc *stateHandleContext) Replace(expected cid.Cid, obj cbor.Marshaler) cid.Cid {
	actr := shc.loadActor()
	if !actr.Head.Equals(expected) {
		panic(fmt.Errorf("unexpected prior stateView %s for actor %s, expected %s", actr.Head, shc.msg.to, expected))
	}
	c := shc.store().StorePut(obj)
	actr.Head = enccid.NewCid(c)
	shc.storeActor(actr)
	return c
}

func (shc *stateHandleContext) store() specsruntime.Store {
	return ((*invocationContext)(shc)).Store()
}

func (shc *stateHandleContext) loadActor() *actor.Actor {
	entry, found, err := shc.rt.state.GetActor(shc.rt.context, shc.msg.to)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("failed to find actor %s for stateView", shc.msg.to))
	}
	return entry
}

func (shc *stateHandleContext) storeActor(actr *actor.Actor) {
	err := shc.rt.state.SetActor(shc.rt.context, shc.msg.to, actr)
	if err != nil {
		panic(err)
	}
}

// runtime aborts are trapped by invoke, it will always return an exit code.
func (ctx *invocationContext) invoke() (ret []byte, errcode exitcode.ExitCode) {
	// Checkpoint stateView, for restoration on revert
	// Note that changes prior to invocation (sequence number bump and gas prepayment) persist even if invocation fails.
	err := ctx.rt.snapshot()
	if err != nil {
		panic(err)
	}
	defer ctx.rt.clearSnapshot()

	// Install handler for abort, which rolls back all stateView changes from this and any nested invocations.
	// This is the only path by which a non-OK exit code may be returned.
	defer func() {
		if r := recover(); r != nil {

			if err := ctx.rt.revert(); err != nil {
				panic(err)
			}
			switch r.(type) {
			case runtime.ExecutionPanic:
				p := r.(runtime.ExecutionPanic)

				vmlog.Warnw("Abort during actor execution.",
					"errorMessage", p,
					"exitCode", p.Code(),
					"sender", ctx.msg.from,
					"receiver", ctx.msg.to,
					"methodNum", ctx.msg.method,
					"value", ctx.msg.value,
					"gasLimit", ctx.gasTank.gasAvailable)
				ret = []byte{} // The Empty here should never be used, but slightly safer than zero value.
				errcode = p.Code()
			default:
				if err, ok := r.(error); ok {
					fmt.Println(err.Error())
				} else {
					fmt.Println(r)
				}

				errcode = 1
				ret = []byte{}
				// do not trap unknown panics
				vmlog.Errorf("spec actors failure: %s", r)
				//debug.PrintStack()
			}
		}
	}()

	// pre-dispatch
	// 1. charge gas for message invocation
	// 2. load target actor
	// 3. transfer optional funds
	// 4. short-circuit _Send_ method
	// 5. load target actor code
	// 6. create target stateView handle
	// assert from address is an ID address.
	if ctx.msg.from.Protocol() != address.ID {
		panic("bad code: sender address MUST be an ID address at invocation time")
	}

	// 1. load target actor
	// Note: we replace the "to" address with the normalized version
	ctx.toActor, ctx.msg.to = ctx.resolveTarget(ctx.msg.to)

	// 2. charge gas for msg
	ctx.gasTank.Charge(ctx.rt.pricelist.OnMethodInvocation(ctx.msg.value, ctx.msg.method), "method invocation")

	// 3. transfer funds carried by the msg
	if !ctx.msg.value.Nil() && !ctx.msg.value.IsZero() {
		if ctx.msg.from != ctx.msg.to {
			ctx.fromActor, ctx.toActor = ctx.rt.transfer(ctx.msg.from, ctx.msg.to, ctx.msg.value)
		}
	}

	// 4. if we are just sending funds, there is nothing else to do.
	if ctx.msg.method == builtin.MethodSend {
		return nil, exitcode.Ok
	}

	// 5. load target actor code
	actorImpl := ctx.rt.getActorImpl(ctx.toActor.Code.Cid)
	// 6. create target stateView handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx))
	ctx.stateHandle = &stateHandle

	// dispatch
	adapter := newRuntimeAdapter(ctx) //runtimeAdapter{ctx: ctx}
	var extErr *dispatch.ExcuteError
	ret, extErr = actorImpl.Dispatch(ctx.msg.method, adapter, ctx.msg.params)
	if extErr != nil {
		runtime.Abortf(extErr.ExitCode(), extErr.Error())
	}

	// post-dispatch
	// 1. check caller was validated
	// 2. check stateView manipulation was valid
	// 4. success!

	// 1. check caller was validated
	if !ctx.isCallerValidated {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Caller MUST be validated during method execution")
	}

	// 2. validate stateView access
	ctx.stateHandle.Validate(func(obj interface{}) cid.Cid {
		id, err := ctx.rt.store.CidOf(obj)
		if err != nil {
			panic(err)
		}
		return id
	})

	// Reset to pre-invocation stateView
	ctx.toActor = nil
	ctx.stateHandle = nil

	// 3. success!
	return ret, exitcode.Ok
}

// resolveTarget loads and actor and returns its ActorID address.
//
// If the target actor does not exist, and the target address is a pub-key address,
// a new account actor will be created.
// Otherwise, this method will abort execution.
func (ctx *invocationContext) resolveTarget(target address.Address) (*actor.Actor, address.Address) {
	// resolve the target address via the InitActor, and attempt to load stateView.
	initActorEntry, found, err := ctx.rt.state.GetActor(ctx.rt.context, builtin.InitActorAddr)
	if err != nil {
		panic(err)
	}
	if !found {
		runtime.Abort(exitcode.SysErrSenderInvalid)
	}

	if target == builtin.InitActorAddr {
		return initActorEntry, target
	}

	// get a view into the actor stateView
	var state init_.State
	if err := ctx.rt.store.Get(ctx.rt.context, initActorEntry.Head.Cid, &state); err != nil {
		panic(err)
	}

	// lookup the ActorID based on the address

	targetActor, found, err := ctx.rt.state.GetActor(ctx.rt.context, target)
	if err != nil {
		panic(err)
	}
	if !found {
		// Charge gas now that easy checks are done
		ctx.gasTank.Charge(gas.PricelistByEpoch(ctx.rt.CurrentEpoch()).OnCreateActor(), "CreateActor  address %s", target)
		// actor does not exist, create an account actor
		// - precond: address must be a pub-key
		// - sent init actor a msg to create the new account
		targetIDAddr, err := ctx.rt.state.RegisterNewAddress(target)
		if err != nil {
			ctx.rt.state.RegisterNewAddress(target)
			panic(err)
		}

		if target.Protocol() != address.SECP256K1 && target.Protocol() != address.BLS {
			// Don't implicitly create an account actor for an address without an associated key.
			runtime.Abort(exitcode.SysErrInvalidReceiver)
		}

		ctx.CreateActor(builtin.AccountActorCodeID, targetIDAddr)

		// call constructor on account
		newMsg := internalMessage{
			from:   builtin.SystemActorAddr,
			to:     targetIDAddr,
			value:  big.Zero(),
			method: builtin.MethodsAccount.Constructor,
			// use original address as constructor params
			// Note: constructor takes a pointer
			params: &target,
		}

		newCtx := newInvocationContext(ctx.rt, ctx.topLevel, newMsg, nil, ctx.gasTank, ctx.randSource)
		_, code := newCtx.invoke()
		if code.IsError() {
			// we failed to construct an account actor..
			runtime.Abort(code)
		}

		// load actor
		//fmt.Println("create account  ", target)
		targetActor, _, err := ctx.rt.state.GetActor(ctx.rt.context, target)
		if err != nil {
			panic(err)
		}
		return targetActor, targetIDAddr
	} else {
		//load id address
		targetIDAddr, found, err := state.ResolveAddress(ctx.rt.ContextStore(), target)
		if err != nil {
			panic(err)
		}

		if !found {
			panic(fmt.Errorf("unreachable: actor is supposed to exist but it does not. addr: %s, idAddr: %s", target, targetIDAddr))
		}

		// load actor
		targetActor, found, err = ctx.rt.state.GetActor(ctx.rt.context, targetIDAddr)
		if err != nil {
			panic(err)
		}

		if !found {
			runtime.Abort(exitcode.SysErrInvalidReceiver)
		}

		return targetActor, targetIDAddr
	}
}

//
// implement runtime.InvocationContext for invocationContext
//

var _ runtime.InvocationContext = (*invocationContext)(nil)

// Runtime implements runtime.InvocationContext.
func (ctx *invocationContext) Runtime() runtime.Runtime {
	return ctx.rt
}

// Store implements runtime.Runtime.
func (ctx *invocationContext) Store() specsruntime.Store {
	return NewActorStorage(ctx.rt.context, ctx.rt.store, ctx.gasTank, ctx.rt.pricelist)
}

// Message implements runtime.InvocationContext.
func (ctx *invocationContext) Message() specsruntime.Message {
	return ctx.msg
}

// ValidateCaller implements runtime.InvocationContext.
func (ctx *invocationContext) ValidateCaller(pattern runtime.CallerPattern) {
	if ctx.isCallerValidated {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Method must validate caller identity exactly once")
	}
	if !pattern.IsMatch((*patternContext2)(ctx)) {
		runtime.Abortf(exitcode.SysErrForbidden, "Method invoked by incorrect caller")
	}
	ctx.isCallerValidated = true
}

// state implements runtime.InvocationContext.
func (ctx *invocationContext) State() specsruntime.StateHandle {
	return ctx.stateHandle
}

// Send implements runtime.InvocationContext.
func (ctx *invocationContext) Send(toAddr address.Address, methodNum abi.MethodNum, params cbor.Marshaler, value abi.TokenAmount, out cbor.Er) exitcode.ExitCode {
	// check if side-effects are allowed
	if !ctx.allowSideEffects {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Calling Send() is not allowed during side-effect lock")
	}
	// prepare
	// 1. alias fromActor
	// 2. build internal message

	// 1. fromActor = executing toActor
	from := ctx.msg.to
	fromActor := ctx.toActor

	// 2. build internal message
	newMsg := internalMessage{
		from:   from,
		to:     toAddr,
		value:  value,
		method: methodNum,
		params: params,
	}

	// invoke
	// 1. build new context
	// 2. invoke message

	// 1. build new context
	newCtx := newInvocationContext(ctx.rt, ctx.topLevel, newMsg, fromActor, ctx.gasTank, ctx.randSource)

	// 2. invoke
	ret, code := newCtx.invoke()
	if code != 0 {
		return code
	} else {
		_ = ctx.gasTank.TryCharge(gasOnActorExec)
		if err := out.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
			runtime.Abortf(exitcode.ErrSerialization, "failed to unmarshal return value: %s", err)
		}
		return code
	}
}

/// Balance implements runtime.InvocationContext.
func (ctx *invocationContext) Balance() abi.TokenAmount {
	return ctx.toActor.Balance
}

//
// implement runtime.InvocationContext for invocationContext
//

var _ runtime.ExtendedInvocationContext = (*invocationContext)(nil)

func (ctx *invocationContext) NewActorAddress() address.Address {
	var buf bytes.Buffer

	b1, err := encoding.Encode(ctx.topLevel.originatorStableAddress)
	if err != nil {
		panic(err)
	}
	_, err = buf.Write(b1)
	if err != nil {
		panic(err)
	}

	err = binary.Write(&buf, binary.BigEndian, ctx.topLevel.originatorCallSeq)
	if err != nil {
		panic(err)
	}

	err = binary.Write(&buf, binary.BigEndian, ctx.topLevel.newActorAddressCount)
	if err != nil {
		panic(err)
	}

	actorAddress, err := address.NewActorAddress(buf.Bytes())
	if err != nil {
		panic(err)
	}
	return actorAddress
}

// CreateActor implements runtime.ExtendedInvocationContext.
// Creating an account is divided into two situations:
//       case1: create based on message receive , the gas fee is deducted first.
//       case2: create from spec actors, check first, and deduct gas fee
func (ctx *invocationContext) CreateActor(codeID cid.Cid, addr address.Address) {
	if !builtin.IsBuiltinActor(codeID) {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Can only create built-in actors.")
	}

	vmlog.Debugf("creating actor, friendly-name: %s, code: %s, addr: %s\n", builtin.ActorNameByCode(codeID), codeID, addr)

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	_, found, err := ctx.rt.state.GetActor(ctx.rt.context, addr)
	if err != nil {
		panic(err)
	}
	if found {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Actor address already exists")
	}

	newActor := &actor.Actor{
		// make this the right 'type' of actor
		Code:       enccid.NewCid(codeID),
		Balance:    abi.NewTokenAmount(0),
		Head:       enccid.NewCid(EmptyObjectCid),
		CallSeqNum: 0,
	}
	if err := ctx.rt.state.SetActor(ctx.rt.context, addr, newActor); err != nil {
		panic(err)
	}

	_ = ctx.gasTank.TryCharge(gasOnActorExec)
}

// DeleteActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) DeleteActor(beneficiary address.Address) {
	receiver := ctx.msg.to
	receiverActor, found, err := ctx.rt.state.GetActor(ctx.rt.context, receiver)
	if err != nil {
		panic(err)
	}

	if !found {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "delete non-existent actor %s", receiverActor)
	}

	ctx.gasTank.Charge(ctx.rt.pricelist.OnDeleteActor(), "DeleteActor %s", receiver)

	// Transfer any remaining balance to the beneficiary.
	// This looks like it could cause a problem with gas refund going to a non-existent actor, but the gas payer
	// is always an account actor, which cannot be the receiver of this message.
	if receiverActor.Balance.GreaterThan(big.Zero()) {
		ctx.fromActor, ctx.toActor = ctx.rt.transfer(receiver, beneficiary, receiverActor.Balance)
	}

	if err := ctx.rt.state.DeleteActor(ctx.rt.context, receiver); err != nil {
		panic(err)
	}

	_ = ctx.gasTank.TryCharge(gasOnActorExec)
}

// patternContext implements the PatternContext
type patternContext2 invocationContext

var _ runtime.PatternContext = (*patternContext2)(nil)

func (ctx *patternContext2) CallerCode() cid.Cid {
	return ctx.fromActor.Code.Cid
}

func (ctx *patternContext2) CallerAddr() address.Address {
	return ctx.msg.from
}
