package msg

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Abstracts over a store of blockchain state.
type previewerChainReader interface {
	GetHead() block.TipSetKey
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

type messagePreviewer interface {
	// PreviewQueryMethod estimates the amount of gas that will be used by a method
	PreviewQueryMethod(ctx context.Context, st state.Tree, vms vm.StorageMap, to address.Address, method types.MethodID, params []byte, from address.Address, optBh *types.BlockHeight) (types.GasUnits, error)
}

// Previewer calculates the amount of Gas needed for a command
type Previewer struct {
	// To get the head tipset state root.
	chainReader previewerChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
	// To to preview messages
	processor messagePreviewer
}

// NewPreviewer constructs a Previewer.
func NewPreviewer(chainReader previewerChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore, processor messagePreviewer) *Previewer {
	return &Previewer{chainReader, cst, bs, processor}
}

// Preview sends a read-only message to an actor.
func (p *Previewer) Preview(ctx context.Context, optFrom, to address.Address, method types.MethodID, params ...interface{}) (types.GasUnits, error) {
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
	usedGas, err := p.processor.PreviewQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "query method returned an error")
	}
	return usedGas, nil
}
