package core

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

// Send executes a message pass inside the VM.
func Send(ctx context.Context, from, to *types.Actor, msg *types.Message, st *types.StateTree) ([]byte, uint8, error) {
	vmCtx := NewVMContext(from, to, msg, st)

	toExecutable, err := LoadCode(to.Code)
	if err != nil {
		return nil, 1, errors.Wrap(err, "unable to load code for To actor")
	}

	if !hasExport(toExecutable.Exports(), msg.Method) {
		return nil, 1, fmt.Errorf("missing export: %s", msg.Method)
	}

	return MakeTypedExport(toExecutable, msg.Method)(vmCtx)
}

func hasExport(exports Exports, method string) bool {
	for m := range exports {
		if m == method {
			return true
		}
	}
	return false
}
