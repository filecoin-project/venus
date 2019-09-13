package consensus

import (
	"context"

	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
)

// Abstracts over a store of blockchain state.
type chainStateChainReader interface {
	GetHead() types.TipSetKey
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

// ChainState knows how to send read-only messages for querying actor state.
type ChainState struct {
	// To get the head tipset state root.
	chainReader chainStateChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewChainState constructs a ChainState.
func NewChainState(chainReader chainStateChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore) *ChainState {
	return &ChainState{chainReader, cst, bs}
}

// ChainStateQueryer queries the chain at a particular tipset
type ChainStateQueryer struct {
	st     state.Tree
	vms    vm.StorageMap
	height *types.BlockHeight
}

// Queryer returns a query interface to query the chain at a particular tipset
func (cs ChainState) Queryer(ctx context.Context, baseKey types.TipSetKey) (ChainStateQueryer, error) {
	st, err := cs.chainReader.GetTipSetState(ctx, baseKey)
	if err != nil {
		return ChainStateQueryer{}, errors.Wrapf(err, "failed to load tree for the state root of tipset: %s", baseKey.String())
	}
	base, err := cs.chainReader.GetTipSet(baseKey)
	if err != nil {
		return ChainStateQueryer{}, errors.Wrapf(err, "failed to get tipset: %s", baseKey.String())
	}
	h, err := base.Height()
	if err != nil {
		return ChainStateQueryer{}, errors.Wrap(err, "failed to get the head tipset height")
	}

	return ChainStateQueryer{
		st:     st,
		vms:    vm.NewStorageMap(cs.bs),
		height: types.NewBlockHeight(h),
	}, nil
}

// Query sends a read-only message against the state of the provided base tipset.
func (q *ChainStateQueryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode message params")
	}

	r, ec, err := CallQueryMethod(ctx, q.st, q.vms, to, method, encodedParams, optFrom, q.height)
	if err != nil {
		return nil, errors.Wrap(err, "querymethod returned an error")
	} else if ec != 0 {
		return nil, errors.Errorf("querymethod returned a non-zero error code %d", ec)
	}
	return r, nil
}
