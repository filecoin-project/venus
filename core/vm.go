package core

import (
	"context"

	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// Send executes a message pass inside the VM. If error is set it
// will always satisfy either ShouldRevert() or IsFault().
func Send(ctx context.Context, from, to *types.Actor, msg *types.Message, st state.Tree) ([]byte, uint8, error) {
	deps := sendDeps{
		transfer: transfer,
	}

	return send(ctx, deps, from, to, msg, st)
}

type sendDeps struct {
	transfer func(*types.Actor, *types.Actor, *types.TokenAmount) error
}

// send executes a message pass inside the VM. It exists alongside Send so that we can inject its dependencies during test.
func send(ctx context.Context, deps sendDeps, from, to *types.Actor, msg *types.Message, st state.Tree) ([]byte, uint8, error) {
	vmCtx := NewVMContext(from, to, msg, st)

	if msg.Value != nil {
		if err := deps.transfer(from, to, msg.Value); err != nil {
			return nil, 1, err
		}
	}

	// save balance changes
	if err := st.SetActor(ctx, msg.From, from); err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "could not set from actor after send")
	}
	if err := st.SetActor(ctx, msg.To, to); err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "could not set to actor after send")
	}

	if msg.Method == "" {
		// if only tokens are transferred there is no need for a method
		// this means we can shortcircuit execution
		return nil, 0, nil
	}

	toExecutable, err := st.GetBuiltinActorCode(to.Code)
	if err != nil {
		return nil, 1, errors.FaultErrorWrap(err, "unable to load code for To actor")
	}

	if !toExecutable.Exports().Has(msg.Method) {
		return nil, 1, errors.NewRevertErrorf("missing export: %s", msg.Method)
	}

	return MakeTypedExport(toExecutable, msg.Method)(vmCtx)
}

func transfer(fromActor, toActor *types.Actor, value *types.TokenAmount) error {
	if value.IsNegative() {
		return ErrCannotTransferNegativeValue
	}

	if fromActor.Balance.LessThan(value) {
		return ErrInsufficientBalance
	}

	if toActor.Balance == nil {
		toActor.Balance = types.ZeroToken // This would be unsafe if TokenAmount could be mutated.
	}
	fromActor.Balance = fromActor.Balance.Sub(value)
	toActor.Balance = toActor.Balance.Add(value)

	return nil
}
