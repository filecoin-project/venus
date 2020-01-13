// Package vmcontext is the internal implementation of the runtime package.
//
// Actors see the interfaces defined in the `runtime` while the concrete implementation is defined here.
package vmcontext

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/exitcode"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
	"github.com/ipfs/go-cid"
)

// ExecutableActorLookup provides a method to get an executable actor by code and protocol version
type ExecutableActorLookup interface {
	GetActorCode(code cid.Cid, version uint64) (dispatch.ExecutableActor, error)
}

// VMContext is the only thing exposed to an actor while executing.
// All methods on the VMContext are ABI methods exposed to actors.
type VMContext struct {
	from              *actor.Actor
	to                *actor.Actor
	toAddr            address.Address
	message           *types.UnsignedMessage
	originMsg         *types.UnsignedMessage
	state             *state.CachedTree
	storageMap        storagemap.StorageMap
	gasTracker        *gastracker.LegacyGasTracker
	blockHeight       *types.BlockHeight
	ancestors         []block.TipSet
	actors            ExecutableActorLookup
	isCallerValidated bool
	allowSideEffects  bool
	stateHandle       actorStateHandle
	blockMiner        address.Address

	deps *deps // Inject external dependencies so we can unit test robustly.
}

// NewContextParams is passed to NewVMContext to construct a new context.
type NewContextParams struct {
	From        *actor.Actor
	To          *actor.Actor
	ToAddr      address.Address
	Message     *types.UnsignedMessage
	OriginMsg   *types.UnsignedMessage
	State       *state.CachedTree
	StorageMap  storagemap.StorageMap
	GasTracker  *gastracker.LegacyGasTracker
	BlockHeight *types.BlockHeight
	Ancestors   []block.TipSet
	Actors      ExecutableActorLookup
	BlockMiner  address.Address
}

// NewVMContext returns an initialized context.
func NewVMContext(params NewContextParams) *VMContext {
	ctx := VMContext{
		from:              params.From,
		to:                params.To,
		toAddr:            params.ToAddr,
		message:           params.Message,
		originMsg:         params.OriginMsg,
		state:             params.State,
		storageMap:        params.StorageMap,
		gasTracker:        params.GasTracker,
		blockHeight:       params.BlockHeight,
		ancestors:         params.Ancestors,
		actors:            params.Actors,
		isCallerValidated: false,
		allowSideEffects:  true,
		blockMiner:        params.BlockMiner,
		deps:              makeDeps(params.State),
	}
	ctx.stateHandle = newActorStateHandle(&ctx, ctx.to.Head)
	return &ctx
}

// GasUnits retrieves the gas cost so far
func (ctx *VMContext) GasUnits() types.GasUnits {
	return ctx.gasTracker.GasConsumedByMessage()
}

var _ runtime.Runtime = (*VMContext)(nil)

// CurrentEpoch is the current chain epoch.
func (ctx *VMContext) CurrentEpoch() types.BlockHeight {
	return *ctx.blockHeight
}

// Randomness gives the actors access to sampling peudo-randomess from the chain.
func (ctx *VMContext) Randomness(epoch types.BlockHeight, offset uint64) runtime.Randomness {
	rnd, err := sampling.SampleChainRandomness(&epoch, ctx.ancestors)
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "failed to sample randomness")
	}
	return rnd
}

// LegacySend allows actors to invoke methods on other actors
func (ctx *VMContext) LegacySend(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	// check if side-effects are allowed
	if !ctx.allowSideEffects {
		runtime.Abortf(exitcode.MethodAbort, "Calling Send() is not allowed during side-effet lock")
	}

	deps := ctx.deps

	// the message sender is the `to` actor, so this is what we set as `from` in the new message
	from := ctx.toAddr
	fromActor := ctx.to

	vals, err := deps.ToValues(params)
	if err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "failed to convert inputs to abi values")
	}

	paramData, err := deps.EncodeValues(vals)
	if err != nil {
		return nil, 1, errors.RevertErrorWrap(err, "encoding params failed")
	}

	msg := types.NewUnsignedMessage(from, to, 0, value, method, paramData)
	if msg.From == msg.To {
		// TODO 3647: handle this
		return nil, 1, errors.NewFaultErrorf("unhandled: sending to self (%s)", msg.From)
	}

	// get actor and id address or create a new account actor.
	toActor, toAddr, err := ctx.getOrCreateActor(context.TODO(), ctx.state, msg.To)
	if err != nil {
		return nil, 1, errors.FaultErrorWrapf(err, "failed to get or create To actor %s", msg.To)
	}
	// TODO(fritz) de-dup some of the logic between here and core.Send
	innerParams := NewContextParams{
		From:        fromActor,
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   ctx.originMsg,
		State:       ctx.state,
		StorageMap:  ctx.storageMap,
		GasTracker:  ctx.gasTracker,
		BlockHeight: ctx.blockHeight,
		Ancestors:   ctx.ancestors,
		Actors:      ctx.actors,
	}
	innerCtx := NewVMContext(innerParams)

	out, ret, err := deps.LegacySend(context.Background(), innerCtx)
	if err != nil {
		return nil, ret, err
	}

	// validate state access
	ctx.stateHandle.Validate()

	return out, ret, nil
}

// Send allows actors to invoke methods on other actors
func (ctx *VMContext) Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) interface{} {
	// check if side-effects are allowed
	if !ctx.allowSideEffects {
		runtime.Abortf(exitcode.MethodAbort, "Calling Send() is not allowed during side-effet lock")
	}

	deps := ctx.deps

	// the message sender is the `to` actor, so this is what we set as `from` in the new message
	from := ctx.toAddr
	fromActor := ctx.to

	vals, err := deps.ToValues(params)
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "failed to convert inputs to abi values")
	}

	paramData, err := deps.EncodeValues(vals)
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "encoding params failed")
	}

	msg := types.NewUnsignedMessage(from, to, 0, value, method, paramData)
	if msg.From == msg.To {
		// TODO 3647: handle this
		runtime.Abortf(exitcode.MethodAbort, "unhandled: sending to self (%s)", msg.From)
	}

	// fetch actor and id address, creating actor if necessary
	toActor, toAddr, err := ctx.getOrCreateActor(context.TODO(), ctx.state, msg.To)
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "failed to get or create To actor %s", msg.To)
	}
	// TODO(fritz) de-dup some of the logic between here and core.Send
	innerParams := NewContextParams{
		From:        fromActor,
		To:          toActor,
		ToAddr:      toAddr,
		Message:     msg,
		OriginMsg:   ctx.originMsg,
		State:       ctx.state,
		StorageMap:  ctx.storageMap,
		GasTracker:  ctx.gasTracker,
		BlockHeight: ctx.blockHeight,
		Ancestors:   ctx.ancestors,
		Actors:      ctx.actors,
	}
	innerCtx := NewVMContext(innerParams)

	return deps.Apply(innerCtx)
}

func apply(ctx *VMContext) interface{} {
	filValue := ctx.message.Value
	if !filValue.Equal(types.ZeroAttoFIL) {
		if filValue.IsNegative() {
			runtime.Abortf(exitcode.MethodAbort, "Can not transfer negative FIL value")
		}
		if err := Transfer(ctx.from, ctx.to, filValue); err != nil {
			runtime.Abort(exitcode.InsufficientFunds)
		}
	}

	msg := ctx.message

	if msg.Method == types.SendMethodID {
		// if only tokens are transferred there is no need for a method
		// this means we can shortcircuit execution
		return nil
	}

	if msg.Method == types.InvalidMethodID {
		// your test should not be getting here..
		// Note: this method is not materialized in production but could occur on tests
		panic("trying to execute fake method on the actual VM, fix test")
	}

	// TODO: use chain height based protocol version here (#3360)
	toExecutable, err := ctx.Actors().GetActorCode(ctx.To().Code, 0)
	if err != nil {
		runtime.Abort(exitcode.ActorCodeNotFound)
	}

	exportedFn, ok := makeTypedExport(toExecutable, msg.Method)
	if !ok {
		runtime.Abort(exitcode.InvalidMethod)
	}

	vals, code, err := exportedFn(ctx)

	// Handle legacy codes and errors
	if err != nil {
		runtime.Abortf(exitcode.MethodAbort, "Legacy actor code returned an error: %s", err.Error())
	}

	if code != 0 {
		runtime.Abortf(exitcode.MethodAbort, "Legacy actor code returned with non-zero error code")
	}

	// validate state access
	ctx.stateHandle.Validate()

	if len(vals) > 0 {
		return vals[0]
	}
	return nil
}

var _ runtime.MessageInfo = (*VMContext)(nil)

// BlockMiner is the address for the actor miner who mined the block in which the initial on-chain message appears.
func (ctx *VMContext) BlockMiner() address.Address {
	return ctx.blockMiner
}

// ValueReceived is the amount of FIL received by this actor during this method call.
//
// Note: the value is already been deposited on the actors account and is reflected on the balance.
func (ctx *VMContext) ValueReceived() types.AttoFIL {
	return ctx.message.Value
}

// Caller is the immediate caller to the current executing method.
func (ctx *VMContext) Caller() address.Address {
	return ctx.message.From
}

var _ runtime.InvocationContext = (*VMContext)(nil)

// Runtime exposes some methods on the runtime to the actor.
func (ctx *VMContext) Runtime() runtime.Runtime {
	return ctx
}

// Message contains information available to the actor about the executing message.
func (ctx *VMContext) Message() runtime.MessageInfo {
	return ctx
}

// ValidateCaller validates the caller against a patter.
//
// All actor methods MUST call this method before returning.
func (ctx *VMContext) ValidateCaller(pattern runtime.CallerPattern) {
	if ctx.isCallerValidated {
		runtime.Abortf(exitcode.MethodAbort, "Method must validate caller identity exactly once")
	}
	if !pattern.IsMatch(patternContext{vm: ctx}) {
		runtime.Abortf(exitcode.MethodAbort, "Method invoked by incorrect caller")
	}
	ctx.isCallerValidated = true
}

// StateHandle handles access to the actor state.
func (ctx *VMContext) StateHandle() runtime.ActorStateHandle {
	return &ctx.stateHandle
}

// Balance is the current balance on the current actors account.
//
// Note: the value received for this invocation is already reflected on the balance.
func (ctx *VMContext) Balance() types.AttoFIL {
	return ctx.to.Balance
}

// Storage returns an implementation of the storage module for this context.
func (ctx *VMContext) Storage() runtime.Storage {
	panic("new method, legacy vmcontext wont provide access to")
}

// LegacyStorage returns an implementation of the storage module for this context.
func (ctx *VMContext) LegacyStorage() runtime.LegacyStorage {
	return ctx.storageMap.NewStorage(ctx.toAddr, ctx.to)
}

// Charge attempts to add the given cost to the accrued gas cost of this transaction
func (ctx *VMContext) Charge(cost types.GasUnits) error {
	return ctx.gasTracker.Charge(cost)
}

var _ runtime.ExtendedInvocationContext = (*VMContext)(nil)

func isBuiltinActor(code cid.Cid) bool {
	return code.Equals(types.AccountActorCodeCid) ||
		code.Equals(types.StorageMarketActorCodeCid) ||
		code.Equals(types.InitActorCodeCid) ||
		code.Equals(types.MinerActorCodeCid) ||
		code.Equals(types.BootstrapMinerActorCodeCid) ||
		code.Equals(types.PaymentBrokerActorCodeCid)
}

func isSingletonActor(code cid.Cid) bool {
	return code.Equals(types.StorageMarketActorCodeCid) ||
		code.Equals(types.InitActorCodeCid) ||
		code.Equals(types.PaymentBrokerActorCodeCid)
}

// CreateActor implements the ExtendedInvocationContext interface.
func (ctx *VMContext) CreateActor(actorID types.Uint64, code cid.Cid, params []interface{}) address.Address {
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
		actorAddr, err = computeActorAddress(ctx.originMsg.From, uint64(ctx.originMsg.CallSeqNum))
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
	newActor, _, err := ctx.state.GetOrCreateActor(context.TODO(), idAddr, func() (*actor.Actor, address.Address, error) {
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

	return actorAddr
}

// VerifySignature implemenets the ExtendedInvocationContext interface.
func (*VMContext) VerifySignature(signer address.Address, signature types.Signature, msg []byte) bool {
	return types.IsValidSignature(msg, signer, signature)
}

var _ runtime.LegacyInvocationContext = (*VMContext)(nil)

// LegacyVerifier returns an interface to the proof verification code
func (ctx *VMContext) LegacyVerifier() verification.Verifier {
	return &verification.FFIBackedProofVerifier{}
}

// LegacyMessage retrieves the message associated with this context.
func (ctx *VMContext) LegacyMessage() *types.UnsignedMessage {
	return ctx.message
}

// LegacyAddressForNewActor creates computes the address for a new actor in the same way that ethereum does.
//
// Note: this will not work if we allow the
// creation of multiple contracts in a given invocation (nonce will remain the
// same, resulting in the same address back)
func (ctx *VMContext) LegacyAddressForNewActor() (address.Address, error) {
	return computeActorAddress(ctx.originMsg.From, uint64(ctx.originMsg.CallSeqNum))
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

//
// internal methods not exposed to actors
//

// AllowSideEffects determines wether or not the actor code is allowed to produce side-effects.
//
// At this time, any `Send` to the same or another actor is considered a side-effect.
func (ctx *VMContext) AllowSideEffects(allow bool) {
	ctx.allowSideEffects = allow
}

// ExtendedRuntime has a few extra methods on top of what is exposed to the actors.
type ExtendedRuntime interface {
	runtime.Runtime
	LegacyMessage() *types.UnsignedMessage
	From() *actor.Actor
	To() *actor.Actor
	Actors() ExecutableActorLookup
}

var _ ExtendedRuntime = (*VMContext)(nil)

// Actors returns the executable actors lookup table.
func (ctx *VMContext) Actors() ExecutableActorLookup {
	return ctx.actors
}

// From returns the actor the message originated from.
func (ctx *VMContext) From() *actor.Actor {
	return ctx.from
}

// To returns the actor the message is intended for.
func (ctx *VMContext) To() *actor.Actor {
	return ctx.to
}

var _ ExportContext = (*VMContext)(nil)

// Params implements ExportContext.
func (ctx *VMContext) Params() []byte {
	return ctx.message.Params
}

// Dependency injection setup.

// makeDeps returns a VMContext's external dependencies with their standard values set.
func makeDeps(st *state.CachedTree) *deps {
	deps := deps{
		EncodeValues: abi.EncodeValues,
		LegacySend:   LegacySend,
		ToValues:     abi.ToValues,
		Apply:        apply,
	}
	if st != nil {
		deps.GetActor = st.GetActor
		deps.GetOrCreateActor = st.GetOrCreateActor
	}
	return &deps
}

type deps struct {
	EncodeValues     func([]*abi.Value) ([]byte, error)
	GetActor         func(context.Context, address.Address) (*actor.Actor, error)
	GetOrCreateActor func(context.Context, address.Address, func() (*actor.Actor, address.Address, error)) (*actor.Actor, address.Address, error)
	LegacySend       func(context.Context, *VMContext) ([][]byte, uint8, error)
	Apply            func(*VMContext) interface{}
	ToValues         func([]interface{}) ([]*abi.Value, error)
}

// LegacySend executes a message pass inside the VM. If error is set it
// will always satisfy either ShouldRevert() or IsFault().
func LegacySend(ctx context.Context, vmCtx *VMContext) (out [][]byte, code uint8, err error) {
	return send(ctx, Transfer, vmCtx)
}

// TransferFn is the money transfer function.
type TransferFn = func(*actor.Actor, *actor.Actor, types.AttoFIL) error

// send executes a message pass inside the VM. It exists alongside Send so that we can inject its dependencies during test.
func send(ctx context.Context, transfer TransferFn, vmCtx *VMContext) ([][]byte, uint8, error) {
	msg := vmCtx.LegacyMessage()
	if !msg.Value.Equal(types.ZeroAttoFIL) {
		if err := transfer(vmCtx.From(), vmCtx.To(), msg.Value); err != nil {
			if errors.ShouldRevert(err) {
				return nil, err.(*errors.RevertError).Code(), err
			}
			return nil, 1, err
		}
	}

	if msg.Method == types.SendMethodID {
		// if only tokens are transferred there is no need for a method
		// this means we can shortcircuit execution
		return nil, 0, nil
	}

	if msg.Method == types.InvalidMethodID {
		// your test should not be getting here..
		// Note: this method is not materialized in production but could occur on tests
		panic("trying to execute fake method on the actual VM, fix test")
	}

	if msg.Method == types.ConstructorMethodID && !vmCtx.From().Code.Equals(types.InitActorCodeCid) {
		return nil, 1, errors.NewRevertError("can only construct actor from init actor")
	}

	// TODO: use chain height based protocol version here (#3360)
	toExecutable, err := vmCtx.Actors().GetActorCode(vmCtx.To().Code, 0)
	if err != nil {
		return nil, errors.ErrNoActorCode, errors.Errors[errors.ErrNoActorCode]
	}

	exportedFn, ok := makeTypedExport(toExecutable, msg.Method)
	if !ok {
		return nil, 1, errors.Errors[errors.ErrMissingExport]
	}

	vals, code, err := exportedFn(vmCtx)
	if vals != nil {
		r, err := abi.ToEncodedValues(vals...)
		if err != nil {
			return nil, 1, errors.FaultErrorWrap(err, "failed to marshal output value")
		}

		if r != nil {
			var rv [][]byte
			err = encoding.Decode(r, &rv)
			if err != nil {
				return nil, 1, errors.NewRevertErrorf("method return doesn't decode as array: %s", err)
			}
			return rv, code, err
		}
	}
	return nil, code, err
}

// Transfer transfers the given value between two actors.
func Transfer(fromActor, toActor *actor.Actor, value types.AttoFIL) error {
	if value.IsNegative() {
		return errors.Errors[errors.ErrCannotTransferNegativeValue]
	}

	if fromActor.Balance.LessThan(value) {
		return errors.Errors[errors.ErrInsufficientBalance]
	}

	fromActor.Balance = fromActor.Balance.Sub(value)
	toActor.Balance = toActor.Balance.Add(value)

	return nil
}

// GetOrCreateActor retrieves an actor by first resolving its address. If that fails it will initialize a new account actor
func (ctx *VMContext) getOrCreateActor(c context.Context, st *state.CachedTree, addr address.Address) (*actor.Actor, address.Address, error) {
	// resolve address before lookup
	idAddr, err := ctx.resolveActorAddress(addr)
	if err != nil {
		return nil, address.Undef, err
	}

	if idAddr != address.Undef {
		act, err := ctx.deps.GetActor(c, idAddr)
		return act, idAddr, err
	}

	// this should never fail due to lack of gas since gas doesn't have meaning here
	ctx.Send(address.InitAddress, initactor.ExecMethodID, types.ZeroAttoFIL, []interface{}{types.AccountActorCodeCid, []interface{}{addr}})
	idAddrInt := ctx.Send(address.InitAddress, initactor.GetActorIDForAddressMethodID, types.ZeroAttoFIL, []interface{}{addr})

	id, ok := idAddrInt.(*big.Int)
	if !ok {
		return nil, address.Undef, errors.NewFaultError("non-integer return from GetActorIDForAddress")
	}

	idAddr, err = address.NewIDAddress(id.Uint64())
	if err != nil {
		return nil, address.Undef, err
	}

	act, err := ctx.deps.GetActor(c, idAddr)
	return act, idAddr, err
}

// resolveAddress looks up associated id address if actor address. Otherwise it returns the same address.
func (ctx *VMContext) resolveActorAddress(addr address.Address) (address.Address, error) {
	if addr.Protocol() == address.ID {
		return addr, nil
	}

	init, err := ctx.deps.GetActor(context.TODO(), address.InitAddress)
	if err != nil {
		return address.Undef, err
	}

	vmCtx := NewVMContext(NewContextParams{
		State:      ctx.state,
		StorageMap: ctx.storageMap,
		ToAddr:     address.InitAddress,
		To:         init,
	})
	id, found, err := initactor.LookupIDAddress(vmCtx, addr)
	if err != nil {
		return address.Undef, err
	}

	if !found {
		return address.Undef, nil
	}

	idAddr, err := address.NewIDAddress(id)
	if err != nil {
		return address.Undef, err
	}

	return idAddr, nil
}

func actorAddressFromParam(maybeAddress interface{}) (address.Address, error) {
	addr, ok := maybeAddress.(address.Address)
	if ok {
		return addr, nil
	}

	serialized, ok := maybeAddress.([]byte)
	if ok {
		addrInt, err := abi.Deserialize(serialized, abi.Address)
		if err != nil {
			return address.Undef, err
		}

		return addrInt.Val.(address.Address), nil
	}

	return address.Undef, errors.NewRevertError("address parameter is not an address")
}

// patternContext is a wrapper on a vmcontext to implement the PatternContext
type patternContext struct {
	vm *VMContext
}

var _ runtime.PatternContext = patternContext{}

func (ctx patternContext) Code() cid.Cid {
	return ctx.vm.from.Code
}
