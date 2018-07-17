package vm

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// Context is the only thing exposed to an actor while executing.
// All methods on the Context are ABI methods exposed to actors.
type Context struct {
	from        *types.Actor
	to          *types.Actor
	message     *types.Message
	state       *state.CachedTree
	storage     StorageMap
	blockHeight *types.BlockHeight

	deps *deps // Inject external dependencies so we can unit test robustly.
}

// NewVMContext returns an initialized context.
func NewVMContext(from, to *types.Actor, msg *types.Message, st *state.CachedTree, store StorageMap, bh *types.BlockHeight) *Context {
	return &Context{
		from:        from,
		to:          to,
		message:     msg,
		state:       st,
		storage:     store,
		blockHeight: bh,
		deps:        makeDeps(st),
	}
}

var _ exec.VMContext = (*Context)(nil)

// Storage returns an implementation of the storage module for this context.
func (ctx *Context) Storage() exec.Storage {
	return ctx.storage.NewStorage(ctx.message.To, ctx.to)
}

// Message retrieves the message associated with this context.
func (ctx *Context) Message() *types.Message {
	return ctx.message
}

// ReadStorage reads the storage from the associated to actor.
func (ctx *Context) ReadStorage() []byte {
	stage := ctx.Storage()

	memory, _ := stage.Get(stage.Head())

	out := make([]byte, len(memory))
	copy(out, memory)

	return out
}

// WriteStorage writes to the storage of the associated to actor.
func (ctx *Context) WriteStorage(memory []byte) error {
	stage := ctx.Storage()

	cid, ec := stage.Put(memory)
	if ec != exec.Ok {
		return errors.NewRevertErrorf("Could not stage memory chunk (Error Code: %d)", ec)
	}

	ec = stage.Commit(cid, stage.Head())
	if ec != exec.Ok {
		return errors.NewRevertErrorf("Could not commit actor memory (Error Code: %d)", ec)
	}

	return nil
}

// BlockHeight returns the block height of the block currently being processed
func (ctx *Context) BlockHeight() *types.BlockHeight {
	return ctx.blockHeight
}

// IsFromAccountActor returns true if the message is being sent by an account actor.
func (ctx *Context) IsFromAccountActor() bool {
	return ctx.from.Code != nil && types.AccountActorCodeCid.Equals(ctx.from.Code)
}

// Send sends a message to another actor.
// This method assumes to be called from inside the `to` actor.
func (ctx *Context) Send(to types.Address, method string, value *types.AttoFIL, params []interface{}) ([][]byte, uint8, error) {
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

	toActor, err := deps.GetOrCreateActor(context.TODO(), msg.To, func() (*types.Actor, error) {
		return &types.Actor{}, nil
	})
	if err != nil {
		return nil, 1, errors.FaultErrorWrapf(err, "failed to get or create To actor %s", msg.To)
	}
	// TODO(fritz) de-dup some of the logic between here and core.Send
	innerCtx := NewVMContext(fromActor, toActor, msg, ctx.state, ctx.storage, ctx.blockHeight)
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
func (ctx *Context) AddressForNewActor() (types.Address, error) {
	return computeActorAddress(ctx.message.From, uint64(ctx.from.Nonce))
}

func computeActorAddress(creator types.Address, nonce uint64) (types.Address, error) {
	buf := new(bytes.Buffer)

	if _, err := buf.Write(creator.Bytes()); err != nil {
		return types.Address{}, err
	}

	if err := binary.Write(buf, binary.BigEndian, nonce); err != nil {
		return types.Address{}, err
	}

	hash, err := types.AddressHash(buf.Bytes())
	if err != nil {
		return types.Address{}, err
	}

	return types.NewMainnetAddress(hash), nil
}

// TEMPCreateActor is a temporary method to create actors.
func (ctx *Context) TEMPCreateActor(addr types.Address, act *types.Actor) error {
	// Check existing address. If nothing there, create empty actor.
	newActor, err := ctx.state.GetOrCreateActor(context.TODO(), addr, func() (*types.Actor, error) {
		return &types.Actor{}, nil
	})

	if err != nil {
		return errors.FaultErrorWrap(err, "Error retrieving or creating actor")
	}

	if newActor.Code != nil {
		return errors.NewRevertErrorf("attempt to create actor at address %s but a non-empty actor is already installed", addr.String())
	}

	// upgrade actor and transfer balance.
	newActor.Code = act.Code
	newActor.Nonce = act.Nonce
	newActor.Balance = act.Balance.Add(newActor.Balance)
	newActor.WriteStorage(act.ReadStorage())
	return nil
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
	GetOrCreateActor func(context.Context, types.Address, func() (*types.Actor, error)) (*types.Actor, error)
	Send             func(context.Context, *Context) ([][]byte, uint8, error)
	ToValues         func([]interface{}) ([]*abi.Value, error)
}
