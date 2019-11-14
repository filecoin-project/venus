// Package vmcontext is the internal implementation of the runtime package.
//
// Actors see the interfaces defined in the `runtime` while the concrete implementation is defined here.
package vmcontext

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sampling"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gastracker"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storagemap"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// ExecutableActorLookup provides a method to get an executable actor by code and protocol version
type ExecutableActorLookup interface {
	GetActorCode(code cid.Cid, version uint64) (dispatch.ExecutableActor, error)
}

// VMContext is the only thing exposed to an actor while executing.
// All methods on the VMContext are ABI methods exposed to actors.
type VMContext struct {
	from        *actor.Actor
	to          *actor.Actor
	message     *types.UnsignedMessage
	state       *state.CachedTree
	storageMap  storagemap.StorageMap
	gasTracker  *gastracker.GasTracker
	blockHeight *types.BlockHeight
	ancestors   []block.TipSet
	actors      ExecutableActorLookup

	deps *deps // Inject external dependencies so we can unit test robustly.
}

// NewContextParams is passed to NewVMContext to construct a new context.
type NewContextParams struct {
	From        *actor.Actor
	To          *actor.Actor
	Message     *types.UnsignedMessage
	State       *state.CachedTree
	StorageMap  storagemap.StorageMap
	GasTracker  *gastracker.GasTracker
	BlockHeight *types.BlockHeight
	Ancestors   []block.TipSet
	Actors      ExecutableActorLookup
}

// NewVMContext returns an initialized context.
func NewVMContext(params NewContextParams) *VMContext {
	return &VMContext{
		from:        params.From,
		to:          params.To,
		message:     params.Message,
		state:       params.State,
		storageMap:  params.StorageMap,
		gasTracker:  params.GasTracker,
		blockHeight: params.BlockHeight,
		ancestors:   params.Ancestors,
		actors:      params.Actors,
		deps:        makeDeps(params.State),
	}
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
	// Dragons: the spec has a TODO on how this works
	rnd, err := sampling.SampleChainRandomness(&epoch, ctx.ancestors)
	if err != nil {
		panic("byteme")
	}
	return rnd
}

// Send allows actors to invoke methods on other actors
func (ctx *VMContext) Send(to address.Address, method types.MethodID, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
	deps := ctx.deps

	// the message sender is the `to` actor, so this is what we set as `from` in the new message
	from := ctx.Message().To
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

	toActor, err := deps.GetOrCreateActor(context.TODO(), msg.To, func() (*actor.Actor, error) {
		return &actor.Actor{}, nil
	})
	if err != nil {
		return nil, 1, errors.FaultErrorWrapf(err, "failed to get or create To actor %s", msg.To)
	}
	// TODO(fritz) de-dup some of the logic between here and core.Send
	innerParams := NewContextParams{
		From:        fromActor,
		To:          toActor,
		Message:     msg,
		State:       ctx.state,
		StorageMap:  ctx.storageMap,
		GasTracker:  ctx.gasTracker,
		BlockHeight: ctx.blockHeight,
		Ancestors:   ctx.ancestors,
		Actors:      ctx.actors,
	}
	innerCtx := NewVMContext(innerParams)

	out, ret, err := deps.Send(context.Background(), innerCtx)
	if err != nil {
		return nil, ret, err
	}

	return out, ret, nil
}

var _ runtime.InvocationContext = (*VMContext)(nil)

// Runtime exposes some methods on the runtime to the actor.
func (ctx *VMContext) Runtime() runtime.Runtime {
	return ctx
}

// ValidateCaller validates the caller against a patter.
//
// All actor methods MUST call this method before returning.
func (ctx *VMContext) ValidateCaller(pattern runtime.CallerPattern) {
	// Dragons: this needs to be coded
}

// Caller is the immediate caller to the current executing method.
func (ctx *VMContext) Caller() address.Address {
	return ctx.message.From
}

// StateHandle handles access to the actor state.
func (ctx *VMContext) StateHandle() runtime.ActorStateHandle {
	// Dragons: this needs to be coded and used
	return nil
}

// ValueReceived is the amount of FIL received by this actor during this method call.
//
// Note: the value is already been deposited on the actors account and is reflected on the balance.
func (ctx *VMContext) ValueReceived() types.AttoFIL {
	return ctx.message.Value
}

// Balance is the current balance on the current actors account.
//
// Note: the value received for this invocation is already reflected on the balance.
func (ctx *VMContext) Balance() types.AttoFIL {
	return ctx.to.Balance
}

// Storage returns an implementation of the storage module for this context.
func (ctx *VMContext) Storage() runtime.Storage {
	return ctx.storageMap.NewStorage(ctx.message.To, ctx.to)
}

// Dragons: here just to avoid deleting a lot of lines while we wait for the new Gas Accounting to land
// Charge attempts to add the given cost to the accrued gas cost of this transaction
func (ctx *VMContext) Charge(cost types.GasUnits) error {
	return ctx.gasTracker.Charge(cost)
}

//
// built-in actor needs
//

// CreateNewActor creates and initializes an actor at the given address.
// If the address is occupied by a non-empty actor, this method will fail.
func (ctx *VMContext) CreateNewActor(addr address.Address, code cid.Cid, initializerData interface{}) error {
	// Check existing address. If nothing there, create empty actor.
	newActor, err := ctx.state.GetOrCreateActor(context.TODO(), addr, func() (*actor.Actor, error) {
		return &actor.Actor{}, nil
	})

	if err != nil {
		return errors.FaultErrorWrap(err, "Error retrieving or creating actor")
	}

	if !newActor.Empty() {
		return errors.NewRevertErrorf("attempt to create actor at address %s but a non-empty actor is already installed", addr.String())
	}

	// make this the right 'type' of actor
	newActor.Code = code

	childStorage := ctx.storageMap.NewStorage(addr, newActor)
	// TODO: need to use blockheight derived version (#3360)
	execActor, err := ctx.actors.GetActorCode(code, 0)
	if err != nil {
		return errors.NewRevertErrorf("attempt to create executable actor from non-existent code %s", code.String())
	}

	err = execActor.InitializeState(childStorage, initializerData)
	if err != nil {
		if !errors.ShouldRevert(err) && !errors.IsFault(err) {
			return errors.RevertErrorWrap(err, "Could not initialize actor state")
		}
		return err
	}

	return nil
}

// Verifier returns an interface to the proof verification code
func (ctx *VMContext) Verifier() verification.Verifier {
	return &verification.RustVerifier{}
}

// Dragons: all extras on the meantime

// Message retrieves the message associated with this context.
func (ctx *VMContext) Message() *types.UnsignedMessage {
	return ctx.message
}

// AddressForNewActor creates computes the address for a new actor in the same
// way that ethereum does.  Note that this will not work if we allow the
// creation of multiple contracts in a given invocation (nonce will remain the
// same, resulting in the same address back)
func (ctx *VMContext) AddressForNewActor() (address.Address, error) {
	return computeActorAddress(ctx.message.From, uint64(ctx.from.Nonce))
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


// ExtendedRuntime has a few extra methods on top of what is exposed to the actors.
type ExtendedRuntime interface {
	runtime.Runtime
	Message() *types.UnsignedMessage
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

// Dependency injection setup.

// makeDeps returns a VMContext's external dependencies with their standard values set.
func makeDeps(st *state.CachedTree) *deps {
	deps := deps{
		EncodeValues: abi.EncodeValues,
		Send:         Send,
		ToValues:     abi.ToValues,
	}
	if st != nil {
		deps.GetOrCreateActor = st.GetOrCreateActor
	}
	return &deps
}

type deps struct {
	EncodeValues     func([]*abi.Value) ([]byte, error)
	GetOrCreateActor func(context.Context, address.Address, func() (*actor.Actor, error)) (*actor.Actor, error)
	Send             func(context.Context, ExtendedRuntime) ([][]byte, uint8, error)
	ToValues         func([]interface{}) ([]*abi.Value, error)
}

// Send executes a message pass inside the VM. If error is set it
// will always satisfy either ShouldRevert() or IsFault().
func Send(ctx context.Context, vmCtx ExtendedRuntime) ([][]byte, uint8, error) {
	return send(ctx, Transfer, vmCtx)
}

// TransferFn is the money transfer function.
type TransferFn = func(*actor.Actor, *actor.Actor, types.AttoFIL) error

// send executes a message pass inside the VM. It exists alongside Send so that we can inject its dependencies during test.
func send(ctx context.Context, transfer TransferFn, vmCtx ExtendedRuntime) ([][]byte, uint8, error) {
	msg := vmCtx.Message()
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

	// TODO: use chain height based protocol version here (#3360)
	toExecutable, err := vmCtx.Actors().GetActorCode(vmCtx.To().Code, 0)
	if err != nil {
		return nil, errors.ErrNoActorCode, errors.Errors[errors.ErrNoActorCode]
	}

	exportedFn, ok := makeTypedExport(toExecutable, msg.Method)
	if !ok {
		return nil, 1, errors.Errors[errors.ErrMissingExport]
	}

	r, code, err := exportedFn(vmCtx)
	if r != nil {
		var rv [][]byte
		err = encoding.Decode(r, &rv)
		if err != nil {
			return nil, 1, errors.NewRevertErrorf("method return doesn't decode as array: %s", err)
		}
		return rv, code, err
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
