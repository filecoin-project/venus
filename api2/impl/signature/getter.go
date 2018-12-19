package signature

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/pkg/errors"
)

var (
	// ErrNoMethod is returned by Get when there is no method signature (eg, transfer).
	ErrNoMethod = errors.New("no method")
)

// ChainReadStore is the subset of chain.ReadStore that SignatureGetter needs.
type ChainReadStore interface {
	LatestState(ctx context.Context) (state.Tree, error)
}

// Getter knows how to get actor method signatures.
type Getter struct {
	chainReader ChainReadStore
}

// NewGetter returns a new SignatureGetter. Shocking.
func NewGetter(chainReader ChainReadStore) *Getter {
	return &Getter{chainReader}
}

// Get returns the signature for the given actor and method. The function signature
// is typically used to enable a caller to decode the output of an actor method call (message).
func (sg *Getter) Get(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	if method == "" {
		return nil, ErrNoMethod
	}

	st, err := sg.chainReader.LatestState(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldnt get current state tree")
	}

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

	export, ok := executable.Exports()[method]
	if !ok {
		return nil, fmt.Errorf("missing export: %s", method)
	}

	return export, nil
}
