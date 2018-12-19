package actor

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/pkg/errors"
)

// ChainReadStore is the subset of chain.ReadStore that SignatureGetter needs.
type ChainReadStore interface {
	LatestState(ctx context.Context) (state.Tree, error)
}

// SignatureGetter knows how to get actor method signatures.
type SignatureGetter struct {
	chainReader ChainReadStore
}

// NewSignatureGetter returns a new SignatureGetter. Shocking.
func NewSignatureGetter(chainReader ChainReadStore) *SignatureGetter {
	return &SignatureGetter{chainReader}
}

// Get returns the signature for the given actor's given method. See vm.GetSignature.
func (sg *SignatureGetter) Get(ctx context.Context, actorAddr address.Address, method string) (_ *exec.FunctionSignature, err error) {
	st, err := sg.chainReader.LatestState(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldnt get current state tree")
	}
	sig, err := vm.GetSignature(ctx, st, actorAddr, method)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get signature for method [%s]", method)
	}
	return sig, nil
}
