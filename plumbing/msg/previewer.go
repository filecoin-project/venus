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
type previewerChainReader interface {
	GetHead() types.TipSetKey
	GetTipSetState(context.Context, types.TipSetKey) (state.Tree, error)
	GetTipSet(types.TipSetKey) (types.TipSet, error)
}

// Previewer calculates the amount of Gas needed for a command
type Previewer struct {
	// To get the head tipset state root.
	chainReader previewerChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewPreviewer constructs a Previewer.
func NewPreviewer(chainReader previewerChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Previewer {
	return &Previewer{chainReader, cst, bs}
}

// Preview sends a read-only message to an actor.
func (p *Previewer) Preview(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "failed to encode message params")
	}

	st, err := p.chainReader.GetTipSetState(ctx, p.chainReader.GetHead())
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "failed to load tree for latest state root")
	}
	head, err := p.chainReader.GetTipSet(p.chainReader.GetHead())
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "failed to get head tipset ")
	}
	h, err := head.Height()
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "failed to get head tipset height")
	}

	vms := vm.NewStorageMap(p.bs)
	usedGas, err := consensus.PreviewQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "query method returned an error")
	}
	return usedGas, nil
}
