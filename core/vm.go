package core

import (
	"context"
	"fmt"
	"math/big"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// Send executes a message pass inside the VM.
func Send(ctx context.Context, from, to *types.Actor, msg *types.Message, st types.StateTree) ([]byte, uint8, error) {
	vmCtx := NewVMContext(from, to, msg, st)

	if msg.Value != nil {
		if err := transfer(from, to, msg.Value); err != nil {
			return nil, 1, errors.Wrap(err, "unable to transfer value")
		}
	}

	if msg.Method == "" {
		// if only tokens are transferred there is no need for a method
		// this means we can shortcircuit execution

		return nil, 0, nil
	}

	toExecutable, err := LoadCode(to.Code)
	if err != nil {
		return nil, 1, errors.Wrap(err, "unable to load code for To actor")
	}

	if !toExecutable.Exports().Has(msg.Method) {
		return nil, 1, fmt.Errorf("missing export: %s", msg.Method)
	}

	return MakeTypedExport(toExecutable, msg.Method)(vmCtx)
}

func transfer(from, to *types.Actor, value *big.Int) error {
	if value.Sign() < 0 {
		return fmt.Errorf("can not transfer negative values")
	}

	if from.Balance.Cmp(value) < 0 {
		return fmt.Errorf("from has not enough balance")
	}

	if to.Balance == nil {
		to.Balance = big.NewInt(0)
	}

	from.Balance.Sub(from.Balance, value)
	to.Balance.Add(to.Balance, value)

	return nil
}
