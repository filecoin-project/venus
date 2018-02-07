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

func NewVMContext(from, to *types.Actor, msg *types.Message, st *types.StateTree) *VMContext {
	return &VMContext{
		from:    from,
		to:      to,
		message: msg,
		state:   st,
	}
}

func (ctx *VMContext) Message() *types.Message {
	return ctx.message
}

func (ctx *VMContext) ReadStorage() []byte {
	return ctx.to.ReadStorage()
}

func (ctx *VMContext) WriteStorage(memory []byte) error {
	ctx.to.WriteStorage(memory)
	return ctx.state.SetActor(context.Background(), ctx.message.To(), ctx.to)
}

func (ctx *VMContext) Send(to types.Address, method string, params []interface{}) ([]byte, uint8, error) {
	// from is always currents context to
	from := ctx.Message().To()
	fromActor := ctx.to

	msg := types.NewMessage(from, to, nil, method, params)
	if msg.From() == msg.To() {
		// TODO: handle this
		return nil, 1, fmt.Errorf("unhandled: sending to self (%s)", msg.From())
	}

	toActor, err := ctx.state.GetOrCreateActor(context.Background(), msg.To())
	if err != nil {
		return nil, 1, errors.Wrapf(err, "failed to get or create To actor %s", msg.To())
	}

	vmCtx := NewVMContext(fromActor, toActor, msg, ctx.state)

	toExecutable, err := LoadCode(toActor.Code())
	if err != nil {
		return nil, 1, errors.Wrap(err, "unable to load code for To actor")
	}

	if !hasExport(toExecutable.Exports(), msg.Method()) {
		return nil, 1, fmt.Errorf("missing export: %s", msg.Method())
	}

	return toExecutable.Execute(vmCtx)
}
