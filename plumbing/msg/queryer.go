package msg

import (
	"context"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Abstracts over a store of blockchain state.
type queryerChainReader interface {
	BlockHeight() (uint64, error)
	GetHead() types.TipSetKey
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
}

// Queryer knows how to send read-only messages for querying actor state.
type Queryer struct {
	// To get the head tipset state root.
	chainReader queryerChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewQueryer constructs a Queryer.
func NewQueryer(chainReader queryerChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Queryer {
	return &Queryer{chainReader, cst, bs}
}

// Query sends a read-only message to an actor.
func (q *Queryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "couldnt encode message params")
	}

	st, err := q.chainReader.GetTipSetState(ctx, q.chainReader.GetHead())
	if err != nil {
		return nil, errors.Wrap(err, "could load tree for latest state root")
	}
	h, err := q.chainReader.BlockHeight()
	if err != nil {
		return nil, errors.Wrap(err, "couldnt get base tipset height")
	}

	vms := vm.NewStorageMap(q.bs)
	r, ec, err := consensus.CallQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return nil, errors.Wrap(err, "querymethod returned an error")
	} else if ec != 0 {
		return nil, errors.Errorf("querymethod returned a non-zero error code %d", ec)
	}
	return r, nil
}
