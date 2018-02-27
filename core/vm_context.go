package core

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// VMContext is the only thing exposed to an actor while executing.
// All methods on the VMContext are ABI methods exposed to actors.
type VMContext struct {
	from    *types.Actor
	to      *types.Actor
	message *types.Message
	state   types.StateTree
}

// NewVMContext returns an initialized context.
func NewVMContext(from, to *types.Actor, msg *types.Message, st types.StateTree) *VMContext {
	return &VMContext{
		from:    from,
		to:      to,
		message: msg,
		state:   st,
	}
}

// Message retrieves the message associated with this context.
func (ctx *VMContext) Message() *types.Message {
	return ctx.message
}

// ReadStorage reads the storage from the associated to actor.
func (ctx *VMContext) ReadStorage() []byte {
	return ctx.to.ReadStorage()
}

// WriteStorage writes to the storage of the associated to actor.
func (ctx *VMContext) WriteStorage(memory []byte) error {
	ctx.to.WriteStorage(memory)
	return ctx.state.SetActor(context.Background(), ctx.message.To, ctx.to)
}

// Send sends a message to another actor.
// This method assumes to be called from inside the `to` actor.
func (ctx *VMContext) Send(to types.Address, method string, value *big.Int, params []interface{}) ([]byte, uint8, error) {
	// the message sender is the `to` actor, so this is what we set as `from` in the new message
	from := ctx.Message().To
	fromActor := ctx.to

	msg := types.NewMessage(from, to, value, method, params)
	if msg.From == msg.To {
		// TODO: handle this
		return nil, 1, fmt.Errorf("unhandled: sending to self (%s)", msg.From)
	}

	toActor, err := ctx.state.GetOrCreateActor(context.Background(), msg.To, func() (*types.Actor, error) {
		return NewAccountActor(nil)
	})
	if err != nil {
		return nil, 1, errors.Wrapf(err, "failed to get or create To actor %s", msg.To)
	}

	return Send(context.Background(), fromActor, toActor, msg, ctx.state)
}

// AddressForNewActor creates computes the address for a new actor in the same
// way that ethereum does.  Note that this will not work if we allow the
// creation of multiple contracts in a given invocation (nonce will remain the
// same, resulting in the same address back)
func (ctx *VMContext) AddressForNewActor() types.Address {
	return computeActorAddress(ctx.message.From, ctx.from.Nonce)
}

func computeActorAddress(creator types.Address, nonce uint64) types.Address {
	h := sha256.New()
	h.Write([]byte(creator))
	binary.Write(h, binary.BigEndian, nonce)
	return types.Address(h.Sum(nil))
}
