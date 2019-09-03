package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/account"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/sampling"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// Context is the only thing exposed to an actor while executing.
// All methods on the Context are ABI methods exposed to actors.
type Context struct {
	from        *actor.Actor
	to          *actor.Actor
	message     *types.Message
	state       *state.CachedTree
	storageMap  StorageMap
	gasTracker  *GasTracker
	blockHeight *types.BlockHeight
	ancestors   []types.TipSet

	deps *deps // Inject external dependencies so we can unit test robustly.
}

var _ exec.VMContext = (*Context)(nil)

// NewContextParams is passed to NewVMContext to construct a new context.
type NewContextParams struct {
	From        *actor.Actor
	To          *actor.Actor
	Message     *types.Message
	State       *state.CachedTree
	StorageMap  StorageMap
	GasTracker  *GasTracker
	BlockHeight *types.BlockHeight
	Ancestors   []types.TipSet
}

// NewVMContext returns an initialized context.
func NewVMContext(params NewContextParams) *Context {
	return &Context{
		from:        params.From,
		to:          params.To,
		message:     params.Message,
		state:       params.State,
		storageMap:  params.StorageMap,
		gasTracker:  params.GasTracker,
		blockHeight: params.BlockHeight,
		ancestors:   params.Ancestors,
		deps:        makeDeps(params.State),
	}
}

var _ exec.VMContext = (*Context)(nil)

// Storage returns an implementation of the storage module for this context.
func (ctx *Context) Storage() exec.Storage {
	return ctx.storageMap.NewStorage(ctx.message.To, ctx.to)
}

// Message retrieves the message associated with this context.
func (ctx *Context) Message() *types.Message {
	return ctx.message
}

// Charge attempts to add the given cost to the accrued gas cost of this transaction
func (ctx *Context) Charge(cost types.GasUnits) error {
	return ctx.gasTracker.Charge(cost)
}

// GasUnits retrieves the gas cost so far
func (ctx *Context) GasUnits() types.GasUnits {
	return ctx.gasTracker.gasConsumedByMessage
}

// BlockHeight returns the block height of the block currently being processed
func (ctx *Context) BlockHeight() *types.BlockHeight {
	return ctx.blockHeight
}

// MyBalance returns the balance of the associated actor.
func (ctx *Context) MyBalance() types.AttoFIL {
	return ctx.to.Balance
}

// IsFromAccountActor returns true if the message is being sent by an account actor.
func (ctx *Context) IsFromAccountActor() bool {
	return account.IsAccount(ctx.from)
}

// Send sends a message to another actor.
// This method assumes to be called from inside the `to` actor.
func (ctx *Context) Send(to address.Address, method string, value types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
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

	msg := types.NewMessage(from, to, 0, value, method, paramData)
	if msg.From == msg.To {
		// TODO: handle this
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
	}
	innerCtx := NewVMContext(innerParams)

	out, ret, err := deps.Send(context.Background(), innerCtx)
	if err != nil {
		return nil, ret, err
	}

	return out, ret, nil
}

// AddressForNewActor creates computes the address for a new actor in the same
// way that ethereum does.  Note that this will not work if we allow the
// creation of multiple contracts in a given invocation (nonce will remain the
// same, resulting in the same address back)
func (ctx *Context) AddressForNewActor() (address.Address, error) {
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

// CreateNewActor creates and initializes an actor at the given address.
// If the address is occupied by a non-empty actor, this method will fail.
func (ctx *Context) CreateNewActor(addr address.Address, code cid.Cid, initializerData interface{}) error {
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
	execActor, err := ctx.state.GetBuiltinActorCode(code)
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

// SampleChainRandomness samples randomness from a block's ancestors at the
// given height.
func (ctx *Context) SampleChainRandomness(sampleHeight *types.BlockHeight) ([]byte, error) {
	return sampling.SampleChainRandomness(sampleHeight, ctx.ancestors)
}

// Verifier returns an interface to the proof verification code
func (ctx *Context) Verifier() verification.Verifier {
	return &verification.RustVerifier{}
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
	Send             func(context.Context, *Context) ([][]byte, uint8, error)
	ToValues         func([]interface{}) ([]*abi.Value, error)
}
