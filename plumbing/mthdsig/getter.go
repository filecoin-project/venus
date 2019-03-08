package mthdsig

import (
	"context"
	"fmt"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
)

var (
	// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method")
	// ErrNoActorImpl is returned by Get when the actor implementation doesn't exist, eg
	// the actor address is an empty actor, an address that has received a transfer of FIL
	// but hasn't yet been upgraded to an account actor. (The actor implementation might
	// also genuinely be missing, which is not expected.)
	ErrNoActorImpl = errors.New("no actor implementation")
)

// ChainReadStore is the subset of chain.ReadStore that Getter needs.
type ChainReadStore interface {
	LatestState(ctx context.Context) (state.Tree, error)
}

// Getter knows how to get actor method signatures.
type Getter struct {
	chainReader ChainReadStore
}

// NewGetter returns a new Getter. Shocking.
func NewGetter(chainReader ChainReadStore) *Getter {
	return &Getter{chainReader}
}

// Get returns the signature for the given actor and method. See api description.
func (sg *Getter) Get(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	st, err := sg.chainReader.LatestState(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldnt get current state tree")
	}

	actor, err := st.GetActor(ctx, actorAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get actor")
	} else if actor.Empty() {
		return nil, ErrNoActorImpl
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
