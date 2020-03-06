package vmcontext

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"runtime/debug"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	init_ "github.com/filecoin-project/specs-actors/actors/builtin/init"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type invocationContext struct {
	rt                *VM
	msg               internalMessage
	fromActor         *actor.Actor
	gasTank           *GasTracker
	randSource        crypto.RandomnessSource
	isCallerValidated bool
	allowSideEffects  bool
	toActor           *actor.Actor
	stateHandle       internalActorStateHandle
}

type internalActorStateHandle interface {
	specsruntime.StateHandle
	Validate(func(interface{}) cid.Cid)
}

func newInvocationContext(rt *VM, msg internalMessage, fromActor *actor.Actor, gasTank *GasTracker, randSource crypto.RandomnessSource) invocationContext {
	// Note: the toActor and stateHandle are loaded during the `invoke()`
	return invocationContext{
		rt:  rt,
		msg: msg,
		// Dragons: based on latest changes, it seems we could delete this
		fromActor:         fromActor,
		gasTank:           gasTank,
		randSource:        randSource,
		isCallerValidated: false,
		allowSideEffects:  true,
	}
}

type stateHandleContext invocationContext

func (ctx *stateHandleContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

func (ctx *stateHandleContext) Store() specsruntime.Store {
	return ((*invocationContext)(ctx)).Store()
}

func (ctx *invocationContext) invoke() interface{} {
	// pre-dispatch
	// 1. charge gas for message invocation
	// 2. load target actor
	// 3. transfer optional funds
	// 4. short-circuit _Send_ method
	// 5. load target actor code
	// 6. create target state handle
	// assert from address is an ID address.
	if ctx.msg.from.Protocol() != address.ID {
		panic("bad code: sender address MUST be an ID address at invocation time")
	}

	// 1. charge gas for msg
	ctx.gasTank.Charge(ctx.rt.pricelist.OnMethodInvocation(ctx.msg.value, ctx.msg.method))

	// 2. load target actor
	// Note: we replace the "to" address with the normalized version
	ctx.toActor, ctx.msg.to = ctx.resolveTarget(ctx.msg.to)

	// 3. transfer funds carried by the msg
	ctx.rt.transfer(ctx.msg.from, ctx.msg.to, ctx.msg.value)

	// 4. if we are just sending funds, there is nothing else to do.
	if ctx.msg.method == builtin.MethodSend {
		return nil
	}

	// 5. load target actor code
	actorImpl := ctx.rt.getActorImpl(ctx.toActor.Code.Cid)

	// 6. create target state handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx), ctx.toActor.Head.Cid)
	ctx.stateHandle = &stateHandle

	// dispatch
	adapter := runtimeAdapter{ctx: ctx}
	out, err := actorImpl.Dispatch(ctx.msg.method, &adapter, ctx.msg.params)
	if err != nil {
		// Dragons: this could be a deserialization error too
		runtime.Abort(exitcode.SysErrInvalidMethod)
	}

	// post-dispatch
	// 1. check caller was validated
	// 2. check state manipulation was valid
	// 3. update actor state
	// 4. success!

	// 1. check caller was validated
	if !ctx.isCallerValidated {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Caller MUST be validated during method execution")
	}

	// 2. validate state access
	ctx.stateHandle.Validate(func(obj interface{}) cid.Cid {
		id, err := ctx.rt.store.CidOf(obj)
		if err != nil {
			panic(err)
		}
		return id
	})

	// 3. update actor state
	// we need to load the actor back up in case something changed during execution
	var found bool
	ctx.toActor, found, err = ctx.rt.state.GetActor(ctx.rt.context, ctx.msg.to)
	if err != nil {
		panic(err)
	}
	if !found {
		// Note: this is ok, it means the actor was deleted during the execution of the message
		return out
	}
	// update the head and save it
	ctx.toActor.Head = e.NewCid(stateHandle.head)
	if err := ctx.rt.state.SetActor(ctx.rt.context, ctx.msg.to, ctx.toActor); err != nil {
		panic(err)
	}

	// 4. success! build the receipt
	return out
}

// resolveTarget loads and actor and returns its ActorID address.
//
// If the target actor does not exist, and the target address is a pub-key address,
// a new account actor will be created.
// Otherwise, this method will abort execution.
func (ctx *invocationContext) resolveTarget(target address.Address) (*actor.Actor, address.Address) {
	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, found, err := ctx.rt.state.GetActor(ctx.rt.context, builtin.InitActorAddr)
	if err != nil {
		panic(err)
	}
	if !found {
		runtime.Abort(exitcode.SysErrActorNotFound)
	}

	if target == builtin.InitActorAddr {
		return initActorEntry, target
	}

	// get a view into the actor state
	var state init_.State
	if _, err := ctx.rt.store.Get(ctx.rt.context, initActorEntry.Head.Cid, &state); err != nil {
		panic(err)
	}

	// lookup the ActorID based on the address
	targetIDAddr, err := state.ResolveAddress(ctx.rt.ContextStore(), target)
	// Dragons: move this logic to resolve address
	notFound := (targetIDAddr == target && target.Protocol() != address.ID)
	if err != nil || notFound {
		// Dragons: we should be ble to just call exec on init..

		// actor does not exist, create an account actor
		// - precond: address must be a pub-key
		// - sent init actor a msg to create the new account

		if target.Protocol() != address.SECP256K1 && target.Protocol() != address.BLS {
			// Don't implicitly create an account actor for an address without an associated key.
			runtime.Abort(exitcode.SysErrActorNotFound)
		}

		targetIDAddr, err = state.MapAddressToNewID(ctx.rt.ContextStore(), target)
		if err != nil {
			panic(err)
		}
		// store new state
		initHead, _, err := ctx.rt.store.Put(ctx.rt.context, &state)
		if err != nil {
			panic(err)
		}
		// update init actor
		initActorEntry.Head = e.NewCid(initHead)
		if err := ctx.rt.state.SetActor(ctx.rt.context, builtin.InitActorAddr, initActorEntry); err != nil {
			panic(err)
		}

		// Review: does this guy have to pay gas?
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

		newCtx := newInvocationContext(ctx.rt, newMsg, nil, ctx.gasTank, ctx.randSource)
		newCtx.invoke()
	}

	// load actor
	targetActor, found, err := ctx.rt.state.GetActor(ctx.rt.context, targetIDAddr)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("unreachable: actor is supposed to exist but it does not. %s", err))
	}

	return targetActor, targetIDAddr
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
	return actorStorage{
		context:   ctx.rt.context,
		inner:     ctx.rt.store,
		gasTank:   ctx.gasTank,
		pricelist: ctx.rt.pricelist,
	}
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
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Method invoked by incorrect caller")
	}
	ctx.isCallerValidated = true
}

// State implements runtime.InvocationContext.
func (ctx *invocationContext) State() specsruntime.StateHandle {
	return ctx.stateHandle
}

type returnWrapper struct {
	inner specsruntime.CBORMarshaler
}

func (r returnWrapper) Into(o specsruntime.CBORUnmarshaler) error {
	// TODO: if inner is also a specsruntime.CBORUnmarshaler, overwrite o with inner.
	b := bytes.Buffer{}
	if r.inner == nil {
		return fmt.Errorf("failed to unmarshal nil return")
	}
	err := r.inner.MarshalCBOR(&b)
	if err != nil {
		return err
	}
	err = o.UnmarshalCBOR(&b)
	return err
}

// Send implements runtime.InvocationContext.
func (ctx *invocationContext) Send(toAddr address.Address, methodNum abi.MethodNum, params specsruntime.CBORMarshaler, value abi.TokenAmount) (ret specsruntime.SendReturn, errcode exitcode.ExitCode) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case runtime.ExecutionPanic:
				p := r.(runtime.ExecutionPanic)
				vmlog.Warnw("Abort during method execution.",
					"errorMessage", p,
					"exitCode", p.Code(),
					"receiver", toAddr,
					"methodNum", methodNum,
					"value", value)
				ret = nil
				errcode = p.Code()
				return
			default:
				// do not trap unknown panics
				debug.PrintStack()
				panic(r)
			}
		}
	}()

	// check if side-effects are allowed
	if !ctx.allowSideEffects {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Calling Send() is not allowed during side-effet lock")
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
	// 3. success!

	// 1. build new context
	newCtx := newInvocationContext(ctx.rt, newMsg, fromActor, ctx.gasTank, ctx.randSource)

	// 2. invoke
	out := newCtx.invoke()

	// 3. success!
	var marsh specsruntime.CBORMarshaler
	if out != nil {
		var ok bool
		marsh, ok = out.(specsruntime.CBORMarshaler)
		if !ok {
			runtime.Abortf(exitcode.SysErrorIllegalActor, "Returned value is not a CBORMarshaler")
		}
	}
	return returnWrapper{inner: marsh}, exitcode.Ok
}

/// Balance implements runtime.InvocationContext.
func (ctx *invocationContext) Balance() abi.TokenAmount {
	return ctx.toActor.Balance
}

// Charge implements runtime.InvocationContext.
func (ctx *invocationContext) Charge(cost gas.Unit) error {
	ctx.gasTank.Charge(cost)
	return nil
}

//
// implement runtime.InvocationContext for invocationContext
//

var _ runtime.ExtendedInvocationContext = (*invocationContext)(nil)

// CreateActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) CreateActor(codeID cid.Cid, addr address.Address) {
	if !isBuiltinActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only create built-in actors.")
	}

	if builtin.IsSingletonActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only have one instance of singleton actors.")
	}

	ctx.gasTank.Charge(ctx.rt.pricelist.OnCreateActor())

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	_, found, err := ctx.rt.state.GetActor(ctx.rt.context, addr)
	if err != nil {
		panic(err)
	}
	if found {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Actor address already exists")
	}
	newActor := &actor.Actor{
		// make this the right 'type' of actor
		Code:    e.NewCid(codeID),
		Balance: abi.NewTokenAmount(0),
	}
	if err := ctx.rt.state.SetActor(ctx.rt.context, addr, newActor); err != nil {
		panic(err)
	}
}

// DeleteActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) DeleteActor() {
	ctx.gasTank.Charge(ctx.rt.pricelist.OnDeleteActor())
	if err := ctx.rt.state.DeleteActor(ctx.rt.context, ctx.msg.to); err != nil {
		panic(err)
	}
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

// Dragons: delete once we remove the bootstrap miner
func isBuiltinActor(code cid.Cid) bool {
	return builtin.IsBuiltinActor(code)
}

func computeActorAddress(creator address.Address, nonce uint64) (address.Address, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(creator.Bytes()); err != nil {
		return address.Undef, err
	}

	if err := binary.Write(buf, binary.BigEndian, nonce); err != nil {
		return address.Undef, err
	}

	return address.NewActorAddress(buf.Bytes())
}
