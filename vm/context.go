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
	state       state.Tree
	blockHeight *types.BlockHeight

	deps *deps // Inject external dependencies so we can unit test robustly.
}

// NewVMContext returns an initialized context.
func NewVMContext(from, to *types.Actor, msg *types.Message, st state.Tree, bh *types.BlockHeight) *Context {
	return &Context{
		from:        from,
		to:          to,
		message:     msg,
		state:       st,
		blockHeight: bh,
		deps:        makeDeps(st),
	}
}

var _ exec.VMContext = (*Context)(nil)

// Message retrieves the message associated with this context.
func (ctx *Context) Message() *types.Message {
	return ctx.message
}

// ReadStorage reads the storage from the associated to actor.
func (ctx *Context) ReadStorage() []byte {
	return ctx.to.ReadStorage()
}

// WriteStorage writes to the storage of the associated to actor.
func (ctx *Context) WriteStorage(memory []byte) error {
	ctx.to.WriteStorage(memory)
	return ctx.state.SetActor(context.Background(), ctx.message.To, ctx.to)
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
func (ctx *Context) Send(to types.Address, method string, value *types.TokenAmount, params []interface{}) ([]byte, uint8, error) {
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
	innerCtx := NewVMContext(fromActor, toActor, msg, ctx.state, ctx.blockHeight)
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
	return computeActorAddress(ctx.message.From, ctx.from.Nonce)
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
	// only allow write over empty actor
	existingActor, _ := ctx.state.GetActor(context.TODO(), addr)
	if existingActor != nil {
		if existingActor.Code == nil {
			// upgrade actor and transfer balance.
			act.Balance = act.Balance.Add(existingActor.Balance)
		} else {
			return errors.NewRevertErrorf("attempt to create actor at address %s but a non-empty actor is already installed", addr.String())
		}
	}
	return ctx.state.SetActor(context.TODO(), addr, act)
}

// Dependency injection setup.

// makeDeps returns a VMContext's external dependencies with their standard values set.
func makeDeps(st state.Tree) *deps {
	deps := deps{
		EncodeValues: abi.EncodeValues,
		Send:         Send,
		ToValues:     abi.ToValues,
	}
	if st != nil {
		deps.SetActor = st.SetActor
		deps.GetOrCreateActor = st.GetOrCreateActor
	}
	return &deps
}

type deps struct {
	EncodeValues     func([]*abi.Value) ([]byte, error)
	GetOrCreateActor func(context.Context, types.Address, func() (*types.Actor, error)) (*types.Actor, error)
	Send             func(context.Context, *Context) ([]byte, uint8, error)
	SetActor         func(context.Context, types.Address, *types.Actor) error
	ToValues         func([]interface{}) ([]*abi.Value, error)
}
