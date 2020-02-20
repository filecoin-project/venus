package vmcontext

import (
	"bytes"
	"context"
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
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type invocationContext struct {
	rt                *VM
	msg               internalMessage
	fromActor         *actor.Actor
	gasTank           *GasTracker
	isCallerValidated bool
	allowSideEffects  bool
	toActor           *actor.Actor
	stateHandle       internalActorStateHandle
}

type internalActorStateHandle interface {
	specsruntime.StateHandle
	Validate(func(interface{}) cid.Cid)
}

func newInvocationContext(rt *VM, msg internalMessage, fromActor *actor.Actor, gasTank *GasTracker) invocationContext {
	// Note: the toActor and stateHandle are loaded during the `invoke()`
	return invocationContext{
		rt:  rt,
		msg: msg,
		// Dragons: based on latest changes, it seems we could delete this
		fromActor:         fromActor,
		gasTank:           gasTank,
		isCallerValidated: false,
		allowSideEffects:  true,
	}
}

type stateHandleContext invocationContext

func (ctx *stateHandleContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

func (ctx *stateHandleContext) Store() specsruntime.Store {
	return ctx.rt.Store()
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
	ctx.gasTank.Charge(gascost.OnMethodInvocation(&ctx.msg))

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
	// 3. success!

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

	ctx.toActor.Head = e.NewCid(stateHandle.head)

	// 3. success! build the receipt
	return out
}

// resolveTarget loads and actor and returns its ActorID address.
//
// If the target actor does not exist, and the target address is a pub-key address,
// a new account actor will be created.
// Otherwise, this method will abort execution.
func (ctx *invocationContext) resolveTarget(target address.Address) (*actor.Actor, address.Address) {
	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, err := ctx.rt.state.GetActor(context.Background(), builtin.InitActorAddr)
	if err != nil {
		panic(fmt.Errorf("init actor not found. %s", err))
	}

	if target == builtin.InitActorAddr {
		return initActorEntry, target
	}

	// build state handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx), initActorEntry.Head.Cid)

	// get a view into the actor state
	var state init_.State
	stateHandle.Readonly(&state)

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

		stateHandle.Transaction(&state, func() interface{} {
			targetIDAddr, err = state.MapAddressToNewID(ctx.rt.ContextStore(), target)
			if err != nil {
				panic(err)
			}
			return nil
		})

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

		// Dragons: the system actor doesnt have an actor..
		newCtx := newInvocationContext(ctx.rt, newMsg, nil, ctx.gasTank)
		newCtx.invoke()
	}

	initActorEntry.Head = e.NewCid(stateHandle.head)

	// load actor
	targetActor, err := ctx.rt.state.GetActor(context.Background(), targetIDAddr)
	if err != nil {
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
	a := []byte{}
	b := bytes.NewBuffer(a)

	err := r.inner.MarshalCBOR(b)
	if err != nil {
		return err
	}
	err = o.UnmarshalCBOR(b)
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
	newCtx := newInvocationContext(ctx.rt, newMsg, fromActor, ctx.gasTank)

	// 2. invoke
	out := newCtx.invoke()

	// 3. success!
	marsh, ok := out.(specsruntime.CBORMarshaler)
	if !ok {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Returned value is not a CBORMarshaler")
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

/// CreateActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) CreateActor(codeID cid.Cid, addr address.Address) {
	if !isBuiltinActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only create built-in actors.")
	}

	if builtin.IsSingletonActor(codeID) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only have one instance of singleton actors.")
	}

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	newActor, _, err := ctx.rt.state.GetOrCreateActor(context.TODO(), addr, func() (*actor.Actor, address.Address, error) {
		return &actor.Actor{}, addr, nil
	})

	if err != nil {
		panic(err)
	}

	if !newActor.Empty() {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Actor address already exists")
	}

	newActor.Balance = abi.NewTokenAmount(0)
	// make this the right 'type' of actor
	newActor.Code = e.NewCid(codeID)
}

/// VerifySignature implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) VerifySignature(signer address.Address, signature crypto.Signature, msg []byte) bool {
	return crypto.IsValidSignature(msg, signer, signature)
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
