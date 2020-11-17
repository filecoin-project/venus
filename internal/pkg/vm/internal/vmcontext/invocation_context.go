package vmcontext

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/go-state-types/network"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
	ipfscbor "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/enccid"
	"github.com/filecoin-project/venus/internal/pkg/encoding"
	"github.com/filecoin-project/venus/internal/pkg/specactors"
	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/account"
	init_ "github.com/filecoin-project/venus/internal/pkg/specactors/builtin/init"
	"github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/runtime"
)

var gasOnActorExec = gas.NewGasCharge("OnActorExec", 0, 0)

// Context for a top-level invocation sequence.
type topLevelContext struct {
	originatorStableAddress address.Address // Stable (public key) address of the top-level message sender.
	originatorCallSeq       uint64          // Call sequence number of the top-level message.
	newActorAddressCount    uint64          // Count of calls To NewActorAddress (mutable).
}

// Context for an individual message invocation, including inter-actor sends.
type invocationContext struct {
	vm                *VM
	topLevel          *topLevelContext
	originMsg         VmMessage //msg not trasfer from and to address
	msg               VmMessage // The message being processed
	gasTank           *GasTracker
	randSource        crypto.RandomnessSource
	isCallerValidated bool
	allowSideEffects  bool
	stateHandle       internalActorStateHandle
	gasIpld           ipfscbor.IpldStore
}

type internalActorStateHandle interface {
	specsruntime.StateHandle
	Validate(func(interface{}) cid.Cid)
}

func newInvocationContext(rt *VM, gasIpld ipfscbor.IpldStore, topLevel *topLevelContext, msg VmMessage, gasTank *GasTracker, randSource crypto.RandomnessSource) invocationContext {
	orginMsg := msg
	// Note: the toActor and stateHandle are loaded during the `invoke()`
	resF, ok := rt.normalizeAddress(msg.From)
	if !ok {
		runtime.Abortf(exitcode.SysErrInvalidReceiver, "resolve msg.From [%s] address failed", msg.From)
	}
	msg.From = resF

	if rt.NtwkVersion() > network.Version3 {
		resT, _ := rt.normalizeAddress(msg.To)
		// may be set to undef if recipient doesn't exist yet
		msg.To = resT
	}

	return invocationContext{
		vm:                rt,
		topLevel:          topLevel,
		msg:               msg,
		originMsg:         orginMsg,
		gasTank:           gasTank,
		randSource:        randSource,
		isCallerValidated: false,
		allowSideEffects:  true,
		stateHandle:       nil,
		gasIpld:           gasIpld,
	}
}

type stateHandleContext invocationContext

func (shc *stateHandleContext) AllowSideEffects(allow bool) {
	shc.allowSideEffects = allow
}

func (shc *stateHandleContext) Create(obj cbor.Marshaler) cid.Cid {
	actr := shc.loadActor()
	if actr.Head.Defined() {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "failed To construct actor stateView: already initialized")
	}
	c := shc.store().StorePut(obj)
	actr.Head = enccid.NewCid(c)
	shc.storeActor(actr)
	return c
}

func (shc *stateHandleContext) Load(obj cbor.Unmarshaler) cid.Cid {
	// The actor must be loaded From store every time since the stateView may have changed via a different stateView handle
	// (e.g. in a recursive call).
	actr := shc.loadActor()
	c := actr.Head.Cid
	if !c.Defined() {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "failed To load undefined stateView, must construct first")
	}
	found := shc.store().StoreGet(c, obj)
	if !found {
		panic(fmt.Errorf("failed To load stateView for actor %s, CID %s", shc.msg.To, c))
	}
	return c
}

func (shc *stateHandleContext) Replace(expected cid.Cid, obj cbor.Marshaler) cid.Cid {
	actr := shc.loadActor()
	if !actr.Head.Equals(expected) {
		panic(fmt.Errorf("unexpected prior stateView %s for actor %s, expected %s", actr.Head, shc.msg.To, expected))
	}
	c := shc.store().StorePut(obj)
	actr.Head = enccid.NewCid(c)
	shc.storeActor(actr)
	return c
}

func (shc *stateHandleContext) store() specsruntime.Store {
	return ((*invocationContext)(shc)).Store()
}

func (shc *stateHandleContext) loadActor() *types.Actor {
	entry, found, err := shc.vm.state.GetActor(shc.vm.context, shc.originMsg.To)
	if err != nil {
		panic(err)
	}
	if !found {
		panic(fmt.Errorf("failed To find actor %s for stateView", shc.originMsg.To))
	}
	return entry
}

func (shc *stateHandleContext) storeActor(actr *types.Actor) {
	err := shc.vm.state.SetActor(shc.vm.context, shc.originMsg.To, actr)
	if err != nil {
		panic(err)
	}
}

// runtime aborts are trapped by invoke, it will always return an exit code.
func (ctx *invocationContext) invoke() (ret []byte, errcode exitcode.ExitCode) {
	// Checkpoint stateView, for restoration on revert
	// Note that changes prior To invocation (sequence number bump and gas prepayment) persist even if invocation fails.
	err := ctx.vm.snapshot()
	if err != nil {
		panic(err)
	}
	defer ctx.vm.clearSnapshot()

	// Install handler for abort, which rolls back all stateView changes From this and any nested invocations.
	// This is the only path by which a non-OK exit code may be returned.
	defer func() {
		if r := recover(); r != nil {

			if err := ctx.vm.revert(); err != nil {
				panic(err)
			}
			switch r.(type) {
			case runtime.ExecutionPanic:
				p := r.(runtime.ExecutionPanic)

				vmlog.Warnw("Abort during actor execution.",
					"errorMessage", p,
					"exitCode", p.Code(),
					"sender", ctx.originMsg.From,
					"receiver", ctx.originMsg.To,
					"methodNum", ctx.originMsg.Method,
					"Value", ctx.originMsg.Value,
					"gasLimit", ctx.gasTank.gasAvailable)
				ret = []byte{} // The Empty here should never be used, but slightly safer than zero Value.
				errcode = p.Code()
			default:
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
	// 4. short-circuit _Send_ Method
	// 5. load target actor code
	// 6. create target stateView handle
	// assert From address is an ID address.
	if ctx.msg.From.Protocol() != address.ID {
		panic("bad code: sender address MUST be an ID address at invocation time")
	}

	// 1. load target actor
	// Note: we replace the "To" address with the normalized version
	_, toIDAddr := ctx.resolveTarget(ctx.originMsg.To)
	if ctx.vm.NtwkVersion() > network.Version3 {
		ctx.msg.To = toIDAddr
	}

	// 2. charge gas for msg
	ctx.gasTank.Charge(ctx.vm.pricelist.OnMethodInvocation(ctx.originMsg.Value, ctx.originMsg.Method), "Method invocation")

	// 3. transfer funds carried by the msg
	if !ctx.originMsg.Value.Nil() && !ctx.originMsg.Value.IsZero() {
		if ctx.msg.From != toIDAddr {
			ctx.vm.transfer(ctx.msg.From, toIDAddr, ctx.originMsg.Value)
		}
	}

	// 4. if we are just sending funds, there is nothing else To do.
	if ctx.originMsg.Method == builtin.MethodSend {
		return nil, exitcode.Ok
	}

	// 5. load target actor code
	toActor, found, err := ctx.vm.state.GetActor(ctx.vm.context, ctx.originMsg.To)
	if err != nil || !found {
		panic(xerrors.Errorf("cannt find to actor %v", err))
	}
	actorImpl := ctx.vm.getActorImpl(toActor.Code.Cid, ctx.Runtime())
	// 6. create target stateView handle
	stateHandle := newActorStateHandle((*stateHandleContext)(ctx))
	ctx.stateHandle = &stateHandle

	// dispatch
	adapter := newRuntimeAdapter(ctx) //runtimeAdapter{ctx: ctx}
	var extErr *dispatch.ExcuteError
	ret, extErr = actorImpl.Dispatch(ctx.originMsg.Method, adapter, ctx.originMsg.Params)
	if extErr != nil {
		runtime.Abortf(extErr.ExitCode(), extErr.Error())
	}

	// post-dispatch
	// 1. check caller was validated
	// 2. check stateView manipulation was valid
	// 4. success!

	// 1. check caller was validated
	if !ctx.isCallerValidated {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Caller MUST be validated during Method execution")
	}

	// 2. validate stateView access
	ctx.stateHandle.Validate(func(obj interface{}) cid.Cid {
		id, err := ctx.vm.store.CidOf(obj)
		if err != nil {
			panic(err)
		}
		return id
	})

	// Reset To pre-invocation stateView
	ctx.stateHandle = nil

	// 3. success!
	return ret, exitcode.Ok
}

// resolveTarget loads and actor and returns its ActorID address.
//
// If the target actor does not exist, and the target address is a pub-key address,
// a new account actor will be created.
// Otherwise, this Method will abort execution.
func (ctx *invocationContext) resolveTarget(target address.Address) (*types.Actor, address.Address) {
	// resolve the target address via the InitActor, and attempt To load stateView.
	initActorEntry, found, err := ctx.vm.state.GetActor(ctx.vm.context, init_.Address)
	if err != nil {
		panic(err)
	}
	if !found {
		runtime.Abort(exitcode.SysErrSenderInvalid)
	}

	if target == init_.Address {
		return initActorEntry, target
	}

	// get init state
	state, err := init_.Load(ctx.vm.ContextStore(), initActorEntry)
	if err != nil {
		panic(err)
	}

	// lookup the ActorID based on the address

	_, found, err = ctx.vm.state.GetActor(ctx.vm.context, target)
	if err != nil {
		panic(err)
	}
	//nolint
	if !found {
		// Charge gas now that easy checks are done
		ctx.gasTank.Charge(gas.PricelistByEpoch(ctx.vm.CurrentEpoch()).OnCreateActor(), "CreateActor  address %s", target)
		// actor does not exist, create an account actor
		// - precond: address must be a pub-key
		// - sent init actor a msg To create the new account
		targetIDAddr, err := ctx.vm.state.RegisterNewAddress(target)
		if err != nil {
			panic(err)
		}

		if target.Protocol() != address.SECP256K1 && target.Protocol() != address.BLS {
			// Don't implicitly create an account actor for an address without an associated key.
			runtime.Abort(exitcode.SysErrInvalidReceiver)
		}

		ctx.CreateActor(getAccountCid(specactors.VersionForNetwork(ctx.vm.NtwkVersion())), targetIDAddr)

		// call constructor on account
		newMsg := VmMessage{
			From:   builtin.SystemActorAddr,
			To:     targetIDAddr,
			Value:  big.Zero(),
			Method: account.Methods.Constructor,
			// use original address as constructor Params
			// Note: constructor takes a pointer
			Params: &target,
		}

		newCtx := newInvocationContext(ctx.vm, ctx.gasIpld, ctx.topLevel, newMsg, ctx.gasTank, ctx.randSource)
		_, code := newCtx.invoke()
		if code.IsError() {
			// we failed To construct an account actor..
			runtime.Abort(code)
		}

		// load actor
		targetActor, _, err := ctx.vm.state.GetActor(ctx.vm.context, target)
		if err != nil {
			panic(err)
		}
		return targetActor, targetIDAddr
	} else {
		//load id address
		targetIDAddr, found, err := state.ResolveAddress(target)
		if err != nil {
			panic(err)
		}

		if !found {
			panic(fmt.Errorf("unreachable: actor is supposed To exist but it does not. addr: %s, idAddr: %s", target, targetIDAddr))
		}

		// load actor
		targetActor, found, err := ctx.vm.state.GetActor(ctx.vm.context, targetIDAddr)
		if err != nil {
			panic(err)
		}

		if !found {
			runtime.Abort(exitcode.SysErrInvalidReceiver)
		}

		return targetActor, targetIDAddr
	}
}

func (ctx *invocationContext) resolveToKeyAddr(addr address.Address) (address.Address, error) {
	if addr.Protocol() == address.BLS || addr.Protocol() == address.SECP256K1 {
		return addr, nil
	}

	act, found, err := ctx.vm.state.GetActor(ctx.vm.context, addr)
	if !found || err != nil {
		return address.Undef, xerrors.Errorf("failed to find actor: %s", addr)
	}

	aast, err := account.Load(adt.WrapStore(ctx.vm.context, ctx.vm.store), act)
	if err != nil {
		return address.Undef, xerrors.Errorf("failed to get account actor state for %s: %v", addr, err)
	}

	return aast.PubkeyAddress()
}

//
// implement runtime.InvocationContext for invocationContext
//
var _ runtime.InvocationContext = (*invocationContext)(nil)

// Runtime implements runtime.InvocationContext.
func (ctx *invocationContext) Runtime() runtime.Runtime {
	return ctx.vm
}

// Store implements runtime.Runtime.
func (ctx *invocationContext) Store() specsruntime.Store {
	return NewActorStorage(ctx.vm.context, ctx.vm.store, ctx.gasTank, ctx.vm.pricelist)
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

	from := ctx.msg.To

	// 2. build internal message
	newMsg := VmMessage{
		From:   from,
		To:     toAddr,
		Value:  value,
		Method: methodNum,
		Params: params,
	}

	// 1. build new context
	newCtx := newInvocationContext(ctx.vm, ctx.gasIpld, ctx.topLevel, newMsg, ctx.gasTank, ctx.randSource)
	// 2. invoke
	ret, code := newCtx.invoke()
	if code == 0 {
		_ = ctx.gasTank.TryCharge(gasOnActorExec)
		if err := out.UnmarshalCBOR(bytes.NewReader(ret)); err != nil {
			runtime.Abortf(exitcode.ErrSerialization, "failed To unmarshal return Value: %s", err)
		}
	}
	return code
}

/// Balance implements runtime.InvocationContext.
func (ctx *invocationContext) Balance() abi.TokenAmount {
	toActor, found, err := ctx.vm.state.GetActor(ctx.vm.context, ctx.originMsg.To)
	if err != nil {
		panic(xerrors.Errorf("cannt find to actor %v", err))
	}
	if !found {
		return abi.NewTokenAmount(0)
	}
	return toActor.Balance
}

//
// implement runtime.InvocationContext for invocationContext
//
var _ runtime.ExtendedInvocationContext = (*invocationContext)(nil)

func (ctx *invocationContext) NewActorAddress() address.Address {
	var buf bytes.Buffer
	origin, err := ctx.resolveToKeyAddr(ctx.topLevel.originatorStableAddress)
	if err != nil {
		panic(err)
	}
	b1, err := encoding.Encode(origin)
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
//       case2: create From spec actors, check first, and deduct gas fee
func (ctx *invocationContext) CreateActor(codeID cid.Cid, addr address.Address) {
	if !builtin.IsBuiltinActor(codeID) {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Can only create built-in actors.")
	}

	vmlog.Debugf("creating actor, friendly-name: %s, code: %s, addr: %s\n", builtin.ActorNameByCode(codeID), codeID, addr)

	// Check existing address. If nothing there, create empty actor.
	// Note: we are storing the actors by ActorID *address*
	_, found, err := ctx.vm.state.GetActor(ctx.vm.context, addr)
	if err != nil {
		panic(err)
	}
	if found {
		runtime.Abortf(exitcode.SysErrorIllegalArgument, "Actor address already exists")
	}

	newActor := &types.Actor{
		// make this the right 'type' of actor
		Code:       enccid.NewCid(codeID),
		Balance:    abi.NewTokenAmount(0),
		Head:       enccid.NewCid(EmptyObjectCid),
		CallSeqNum: 0,
	}
	if err := ctx.vm.state.SetActor(ctx.vm.context, addr, newActor); err != nil {
		panic(err)
	}

	_ = ctx.gasTank.TryCharge(gasOnActorExec)
}

// DeleteActor implements runtime.ExtendedInvocationContext.
func (ctx *invocationContext) DeleteActor(beneficiary address.Address) {
	receiver := ctx.originMsg.To
	receiverActor, found, err := ctx.vm.state.GetActor(ctx.vm.context, receiver)
	if err != nil {
		panic(err)
	}

	if !found {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "delete non-existent actor %v", receiverActor)
	}

	ctx.gasTank.Charge(ctx.vm.pricelist.OnDeleteActor(), "DeleteActor %s", receiver)

	// Transfer any remaining balance To the beneficiary.
	// This looks like it could cause a problem with gas refund going To a non-existent actor, but the gas payer
	// is always an account actor, which cannot be the receiver of this message.
	if receiverActor.Balance.GreaterThan(big.Zero()) {
		ctx.vm.transfer(receiver, beneficiary, receiverActor.Balance)
	}

	if err := ctx.vm.state.DeleteActor(ctx.vm.context, receiver); err != nil {
		panic(err)
	}

	_ = ctx.gasTank.TryCharge(gasOnActorExec)
}

func (ctx *invocationContext) stateView() SyscallsStateView {
	// The stateView tree's root is not committed until the end of a tipset, so we can't use the external stateView view
	// type for this implementation.
	// Maybe we could re-work it To use a root HAMT node rather than root CID.
	return newSyscallsStateView(ctx, ctx.vm)
}

// patternContext implements the PatternContext
type patternContext2 invocationContext

var _ runtime.PatternContext = (*patternContext2)(nil)

func (ctx *patternContext2) CallerCode() cid.Cid {
	toActor, found, err := ctx.vm.state.GetActor(ctx.vm.context, ctx.originMsg.From)
	if err != nil || !found {
		panic(xerrors.Errorf("cannt find to actor %v", err))
	}
	return toActor.Code.Cid
}

func (ctx *patternContext2) CallerAddr() address.Address {
	return ctx.msg.From
}
