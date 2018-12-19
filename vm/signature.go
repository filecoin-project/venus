package vm

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
)

var (
	// ErrNoMethod is returned by GetSignature when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method in message")
)

// GetSignature returns the signature for the given actor and method.
func GetSignature(ctx context.Context, st state.Tree, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if !actor.Code.Defined() {
		return nil, fmt.Errorf("no actor implementation")
	}

	executable, err := st.GetBuiltinActorCode(actor.Code)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load actor code")
	}

	if method == "" {
		return nil, ErrNoMethod
	}

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}
