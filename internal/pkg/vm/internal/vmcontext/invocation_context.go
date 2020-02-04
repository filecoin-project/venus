package vmcontext

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/ipfs/go-cid"
)

type invocationContext struct {
	rt                *VM
	msg               internalMessage
	fromActor         *actor.Actor
	gasTank           *gas.Tracker
	isCallerValidated bool
	allowSideEffects  bool
	toActor           *actor.Actor
	stateHandle       internalActorStateHandle
}

type internalActorStateHandle interface {
	runtime.ActorStateHandle
	Validate()
}

func newInvocationContext(rt *VM, msg internalMessage, fromActor *actor.Actor, gasTank *gas.Tracker) invocationContext {
	// Note: the toActor and stateHandle are loaded during the `invoke()`
	return invocationContext{
		rt:                rt,
		msg:               msg,
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

func (ctx *stateHandleContext) Storage() runtime.Storage {
	return ctx.rt.Storage()
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
	runtime.Assert(ctx.msg.from.Protocol() == address.ID)

	// 1. charge gas for msg
	ctx.gasTank.Charge(gascost.OnMethodInvocation(&ctx.msg))

	// 2. load target actor
	// Note: we replace the "to" address with the normalized version
	ctx.toActor, ctx.msg.to = ctx.resolveTarget(ctx.msg.to)

	// 3. transfer funds carried by the msg
	ctx.rt.transfer(ctx.msg.from, ctx.msg.to, ctx.msg.value)

	// 4. if we are just sending funds, there is nothing else to do.
	if ctx.msg.method == types.SendMethodID {
		return message.Ok().WithGas(ctx.gasTank.GasConsumed())
	}

	// 5. load target actor code
	actorImpl := ctx.rt.getActorImpl(ctx.toActor.Code)

	// 6. create target state handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx), ctx.toActor.Head)
	ctx.stateHandle = &stateHandle

	// dispatch
	// 1. check method exists
	// 2. invoke method on actor

	// 1. check method exists
	exportedFn, ok := makeTypedExport(actorImpl, ctx.msg.method)
	if !ok {
		runtime.Abort(exitcode.InvalidMethod)
	}

	// 2. invoke method on actor
	vals, code, err := exportedFn(ctx)

	// handle legacy errors and codes
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "Legacy actor code returned an error. %s", err)
	}
	if code != 0 {
		runtime.Abortf(exitcode.MethodAbort, "Legacy actor code returned with non-zero error code, code: %d", code)
	}

	// post-dispatch
	// 1. check caller was validated
	// 2. check state manipulation was valid
	// 3. success!

	// 1. check caller was validated
	if !ctx.isCallerValidated {
		runtime.Abortf(exitcode.MethodAbort, "Caller MUST be validated during method execution")
	}

	// 2. validate state access
	ctx.stateHandle.Validate()

	ctx.toActor.Head = stateHandle.head

	// 3. success! build the receipt
	if len(vals) > 0 {
		return vals[0]
	}
	return nil
}

// resolveTarget loads and actor and returns its ActorID address.
//
// If the target actor does not exist, and the target address is a pub-key address,
// a new account actor will be created.
// Otherwise, this method will abort execution.
func (ctx *invocationContext) resolveTarget(target address.Address) (*actor.Actor, address.Address) {
	// resolve the target address via the InitActor, and attempt to load state.
	initActorEntry, err := ctx.rt.state.GetActor(context.Background(), address.InitAddress)
	if err != nil {
		panic(fmt.Errorf("init actor not found. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(ctx.rt.Storage(), initActorEntry.Head)

	// get a view into the actor state
	initView := initactor.NewView(stateHandle, ctx.rt.Storage())

	// lookup the ActorID based on the address
	targetIDAddr, ok := initView.GetIDAddressByAddress(target)
	if ok {
		targetActor, err := ctx.rt.state.GetActor(context.Background(), targetIDAddr)
		if err == nil {
			// actor found, return it and its IDAddress
			return targetActor, targetIDAddr
		}
	}

	// actor does not exist, create an account actor
	// - precond: address must be a pub-key
	// - sent init actor a msg to create the new account

	if !target.IsPubKey() {
		// Don't implicitly create an account actor for an address without an associated key.
		runtime.Abort(exitcode.ActorNotFound)
	}

	// send init actor msg to create the account actor
	params := []interface{}{types.AccountActorCodeCid, []interface{}{target}}

	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		runtime.Abortf(exitcode.EncodingError, "failed to encode params. %s", err)
	}
	newMsg := internalMessage{
		from:   ctx.msg.from,
		to:     address.InitAddress,
		value:  types.ZeroAttoFIL,
		method: initactor.ExecMethodID,
		params: encodedParams,
	}

	newCtx := newInvocationContext(ctx.rt, newMsg, ctx.fromActor, ctx.gasTank)

	targetIDAddrOpaque := newCtx.invoke()
	// cast response, interface{} -> address.Address
	targetIDAddr = targetIDAddrOpaque.(address.Address)

	// load actor
	targetActor, err := ctx.rt.state.GetActor(context.Background(), targetIDAddr)
	if err != nil {
		panic(fmt.Errorf("unreachable: exec failed to create the actor but returned successfully. %s", err))
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
func (ctx *invocationContext) Message() runtime.MessageInfo {
	return ctx.msg
}

// ValidateCaller implements runtime.InvocationContext.
func (ctx *invocationContext) ValidateCaller(pattern runtime.CallerPattern) {
	if ctx.isCallerValidated {
		runtime.Abortf(exitcode.MethodAbort, "Method must validate caller identity exactly once")
	}
	if !pattern.IsMatch((*patternContext2)(ctx)) {
		runtime.Abortf(exitcode.MethodAbort, "Method invoked by incorrect caller")
	}
	ctx.isCallerValidated = true
}

// StateHandle implements runtime.InvocationContext.
func (ctx *invocationContext) StateHandle() runtime.ActorStateHandle {
	return ctx.stateHandle
}

// LegacySend implements runtime.InvocationContext.
func (ctx *invocationContext) LegacySend(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	panic("legacy code invoked")
}

// Send implements runtime.InvocationContext.
func (ctx *invocationContext) Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) interface{} {
	// check if side-effects are allowed
	if !ctx.allowSideEffects {
		runtime.Abortf(exitcode.MethodAbort, "Calling Send() is not allowed during side-effet lock")
	}

	// prepare
	// 1. alias fromActor
	// 2. build internal message

	// 1. fromActor = executing toActor
	from := ctx.msg.to
	fromActor := ctx.toActor

	// 2. build internal message
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		runtime.Abortf(exitcode.EncodingError, "failed to encode params. %s", err)
	}
	newMsg := internalMessage{
		from:   from,
		to:     to,
		value:  value,
		method: method,
		params: encodedParams,
	}

	// invoke
	// 1. build new context
	// 2. invoke message
	// 3. success!

	// 1. build new context
	newCtx := newInvocationContext(ctx.rt, newMsg, fromActor, ctx.gasTank)

	// 2. invoke
	ret := newCtx.invoke()

	// 3. success!
	return ret
}

/// Balance implements runtime.InvocationContext.
func (ctx *invocationContext) Balance() types.AttoFIL {
	return ctx.toActor.Balance
}

// Charge implements runtime.InvocationContext.
func (ctx *invocationContext) Charge(cost types.GasUnits) error {
	ctx.gasTank.Charge(cost)
	return nil
}

//
// implement runtime.InvocationContext for invocationContext
//

var _ runtime.ExtendedInvocationContext = (*invocationContext)(nil)

/// CreateActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) CreateActor(actorID types.Uint64, code cid.Cid, params []interface{}) (address.Address, address.Address) {
	// Dragons: code it over, there were some changes in spec, revise
	if !isBuiltinActor(code) {
		runtime.Abortf(exitcode.MethodAbort, "Can only create built-in actors.")
	}

	if isSingletonActor(code) {
		runtime.Abortf(exitcode.MethodAbort, "Can only have one instance of singleton actors.")
	}

	// create address for actor
	var actorAddr address.Address
	var err error
	if types.AccountActorCodeCid.Equals(code) {
		// address for account actor comes from first parameter
		if len(params) < 1 {
			runtime.Abortf(exitcode.MethodAbort, "Missing address parameter for account actor creation")
		}
		actorAddr, err = actorAddressFromParam(params[0])
		if err != nil {
			runtime.Abortf(exitcode.MethodAbort, "Parameter for account actor creation is not an address")
		}
	} else {
		actorAddr, err = computeActorAddress(ctx.msg.from, uint64(ctx.msg.callSeqNumber))
		if err != nil {
			runtime.Abortf(exitcode.MethodAbort, "Could not create address for actor")
		}
	}

	idAddr, err := address.NewIDAddress(uint64(actorID))
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "Could not create IDAddress for actor")
	}

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	newActor, _, err := ctx.rt.state.GetOrCreateActor(context.TODO(), idAddr, func() (*actor.Actor, address.Address, error) {
		return &actor.Actor{}, idAddr, nil
	})

	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "Could not get or create actor")
	}

	if !newActor.Empty() {
		runtime.Abortf(exitcode.MethodAbort, "Actor address already exists")
	}

	// make this the right 'type' of actor
	newActor.Code = code

	// send message containing actor's initial balance to construct it with the given params
	ctx.Send(idAddr, types.ConstructorMethodID, ctx.Message().ValueReceived(), params)

	return idAddr, actorAddr
}

/// VerifySignature implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) VerifySignature(signer address.Address, signature types.Signature, msg []byte) bool {
	return types.IsValidSignature(msg, signer, signature)
}

//
// implement ExportContext for invocationContext
//

var _ ExportContext = (*invocationContext)(nil)

func (ctx *invocationContext) Params() []byte {
	return ctx.msg.params
}

// patternContext implements the PatternContext
type patternContext2 invocationContext

var _ runtime.PatternContext = (*patternContext2)(nil)

func (ctx *patternContext2) Code() cid.Cid {
	return ctx.fromActor.Code
}
