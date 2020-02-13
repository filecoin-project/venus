package vmcontext

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/filecoin-project/go-address"

	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
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
	if ctx.msg.method == types.SendMethodID {
		return message.Ok().WithGas(ctx.gasTank.GasConsumed())
	}

	// 5. load target actor code
	actorImpl := ctx.rt.getActorImpl(ctx.toActor.Code.Cid)

	// 6. create target state handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx), ctx.toActor.Head.Cid)
	ctx.stateHandle = &stateHandle

	// dispatch
	// Dragons: uncomment and send this over when we delete the existing actors and bring the new ones
	// adapter := adapter.NewAdapter(ctx)
	out, err := actorImpl.Dispatch(ctx.msg.method, ctx, ctx.msg.params)
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
	ctx.stateHandle.Validate()

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
	initActorEntry, err := ctx.rt.state.GetActor(context.Background(), vmaddr.InitAddress)
	if err != nil {
		panic(fmt.Errorf("init actor not found. %s", err))
	}

	// build state handle
	var stateHandle = NewReadonlyStateHandle(ctx.rt.Storage(), initActorEntry.Head.Cid)

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

	if target.Protocol() != address.SECP256K1 && target.Protocol() != address.BLS {
		// Don't implicitly create an account actor for an address without an associated key.
		runtime.Abort(exitcode.SysErrActorNotFound)
	}

	// send init actor msg to create the account actor
	params := []interface{}{builtin.AccountActorCodeID, []interface{}{target}}

	encodedParams, err := encoding.Encode(params)
	if err != nil {
		runtime.Abortf(exitcode.SysErrSerialization, "failed to encode params. %s", err)
	}
	newMsg := internalMessage{
		from:   ctx.msg.from,
		to:     vmaddr.InitAddress,
		value:  big.Zero(),
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
func (ctx *invocationContext) State() runtime.ActorStateHandle {
	return ctx.stateHandle
}

// Send implements runtime.InvocationContext.
func (ctx *invocationContext) Send(to address.Address, method types.MethodID, value abi.TokenAmount, params interface{}) interface{} {
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
	encodedParams, ok := params.([]byte)
	if !ok && params != nil {
		var err error
		encodedParams, err = encoding.Encode(params)
		if err != nil {
			runtime.Abortf(exitcode.SysErrSerialization, "failed to encode params. %s", err)
		}
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
func (ctx *invocationContext) CreateActor(actorID types.Uint64, code cid.Cid, constructorParams []byte) (address.Address, address.Address) {
	// Dragons: code it over, there were some changes in spec, revise
	if !isBuiltinActor(code) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only create built-in actors.")
	}

	if isSingletonActor(code) {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Can only have one instance of singleton actors.")
	}

	// create address for actor
	var actorAddr address.Address
	var err error
	if builtin.AccountActorCodeID.Equals(code) {
		err = encoding.Decode(constructorParams, &actorAddr)
		if err != nil {
			runtime.Abortf(exitcode.SysErrorIllegalActor, "Parameter for account actor creation is not an address")
		}
	} else {
		actorAddr, err = computeActorAddress(ctx.msg.from, ctx.msg.callSeqNumber)
		if err != nil {
			panic("Could not create address for actor")
		}
	}

	idAddr, err := address.NewIDAddress(uint64(actorID))
	if err != nil {
		panic("Could not create IDAddress for actor")
	}

	// Check existing address. If nothing there, create empty actor.
	//
	// Note: we are storing the actors by ActorID *address*
	newActor, _, err := ctx.rt.state.GetOrCreateActor(context.TODO(), idAddr, func() (*actor.Actor, address.Address, error) {
		return &actor.Actor{}, idAddr, nil
	})

	if err != nil {
		panic(err)
	}

	if !newActor.Empty() {
		runtime.Abortf(exitcode.ErrIllegalArgument, "Actor address already exists")
	}

	newActor.Balance = abi.NewTokenAmount(0)
	// make this the right 'type' of actor
	newActor.Code = e.NewCid(code)

	// send message containing actor's initial balance to construct it with the given params
	ctx.Send(idAddr, types.ConstructorMethodID, ctx.Message().ValueReceived(), constructorParams)

	return idAddr, actorAddr
}

/// VerifySignature implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) VerifySignature(signer address.Address, signature types.Signature, msg []byte) bool {
	return types.IsValidSignature(msg, signer, signature)
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

// Dragons: move this to specs-actors
func isBuiltinActor(code cid.Cid) bool {
	return code.Equals(builtin.AccountActorCodeID) ||
		code.Equals(builtin.CronActorCodeID) ||
		code.Equals(builtin.InitActorCodeID) ||
		code.Equals(builtin.MultisigActorCodeID) ||
		code.Equals(builtin.PaymentChannelActorCodeID) ||
		code.Equals(builtin.RewardActorCodeID) ||
		code.Equals(builtin.StorageMarketActorCodeID) ||
		code.Equals(builtin.StorageMinerActorCodeID) ||
		code.Equals(builtin.StoragePowerActorCodeID) ||
		code.Equals(types.BootstrapMinerActorCodeCid)
}

// Dragons: move this to specs-actors
func isSingletonActor(code cid.Cid) bool {
	return code.Equals(builtin.CronActorCodeID) ||
		code.Equals(builtin.InitActorCodeID) ||
		code.Equals(builtin.StorageMarketActorCodeID) ||
		code.Equals(builtin.StoragePowerActorCodeID)
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
