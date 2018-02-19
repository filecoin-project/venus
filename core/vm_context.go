package core

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// VMContext is the only thing exposed to an actor while executing.
type VMContext struct {
	from    *types.Actor
	to      *types.Actor
	message *types.Message
	state   *types.StateTree
}

// NewVMContext returns an initialized context.
func NewVMContext(from, to *types.Actor, msg *types.Message, st *types.StateTree) *VMContext {
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
	return ctx.state.SetActor(context.Background(), ctx.message.To(), ctx.to)
}

// Send sends a message to another actor.
func (ctx *VMContext) Send(to types.Address, method string, params []interface{}) ([]byte, uint8, error) {
	// from is always currents context to
	from := ctx.Message().To()
	fromActor := ctx.to

	msg := types.NewMessage(from, to, method, params)
	if msg.From() == msg.To() {
		// TODO: handle this
		return nil, 1, fmt.Errorf("unhandled: sending to self (%s)", msg.From())
	}

	toActor, err := ctx.state.GetOrCreateActor(context.Background(), msg.To())
	if err != nil {
		return nil, 1, errors.Wrapf(err, "failed to get or create To actor %s", msg.To())
	}

	return Send(context.Background(), fromActor, toActor, msg, ctx.state)
}
