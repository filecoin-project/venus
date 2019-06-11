// Package vm implements the Filecoin VM
// This means this is the _only_ part of the code base that should concern itself
// with passing data between VM boundaries.
package vm

import (
	"context"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// Send executes a message pass inside the VM. If error is set it
// will always satisfy either ShouldRevert() or IsFault().
func Send(ctx context.Context, vmCtx *Context) ([][]byte, uint8, error) {
	deps := sendDeps{
		transfer: Transfer,
	}
	return send(ctx, deps, vmCtx)
}

type sendDeps struct {
	transfer func(*actor.Actor, *actor.Actor, types.AttoFIL) error
}

// send executes a message pass inside the VM. It exists alongside Send so that we can inject its dependencies during test.
func send(ctx context.Context, deps sendDeps, vmCtx *Context) ([][]byte, uint8, error) {
	if !vmCtx.message.Value.Equal(types.ZeroAttoFIL) {
		if err := deps.transfer(vmCtx.from, vmCtx.to, vmCtx.message.Value); err != nil {
			if errors.ShouldRevert(err) {
				return nil, err.(*errors.RevertError).Code(), err
			}
			return nil, 1, err
		}
	}

	if vmCtx.message.Method == "" {
		// if only tokens are transferred there is no need for a method
		// this means we can shortcircuit execution
		return nil, 0, nil
	}

	toExecutable, err := vmCtx.state.GetBuiltinActorCode(vmCtx.to.Code)
	if err != nil {
		return nil, errors.ErrNoActorCode, errors.Errors[errors.ErrNoActorCode]
	}

	if !toExecutable.Exports().Has(vmCtx.message.Method) {
		return nil, 1, errors.Errors[errors.ErrMissingExport]
	}

	r, code, err := actor.MakeTypedExport(toExecutable, vmCtx.message.Method)(vmCtx)
	if r != nil {
		var rv [][]byte
		err = cbor.DecodeInto(r, &rv)
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
