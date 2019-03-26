package msg

import (
	"context"

	hamt "github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor/builtin"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// Queryer knows how to send read-only messages for querying actor state.
type Queryer struct {
	// For getting the default address. Lame.
	repo   repo.Repo
	wallet *wallet.Wallet
	// To get the head tipset state root.
	chainReader chain.ReadStore
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewQueryer constructs a Queryer.
func NewQueryer(repo repo.Repo, wallet *wallet.Wallet, chainReader chain.ReadStore, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Queryer {
	return &Queryer{repo, wallet, chainReader, cst, bs}
}

// Query sends a read-only message to an actor.
func (q *Queryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, *exec.FunctionSignature, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldnt encode message params")
	}

	// We return the method signature so callers know how to decode the return value.
	// Probably would be better to do the decoding here since we are after all accepting
	// golang types.
	sigGetter := mthdsig.NewGetter(q.chainReader)
	sig, err := sigGetter.Get(ctx, to, method)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to determine return type")
	}

	headTs := q.chainReader.Head()
	tsas, err := q.chainReader.GetTipSetAndState(ctx, headTs.ToSortedCidSet())
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldnt get latest state root")
	}
	st, err := state.LoadStateTree(ctx, q.cst, tsas.TipSetStateRoot, builtin.Actors)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could load tree for latest state root")
	}
	h, err := headTs.Height()
	if err != nil {
		return nil, nil, errors.Wrap(err, "couldnt get base tipset height")
	}

	vms := vm.NewStorageMap(q.bs)
	r, ec, err := consensus.CallQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return nil, nil, errors.Wrap(err, "querymethod returned an error")
	} else if ec != 0 {
		return nil, nil, errors.Errorf("querymethod returned a non-zero error code %d", ec)
	}
	return r, sig, nil
}
