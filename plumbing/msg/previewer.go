package msg

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm"
	"github.com/filecoin-project/go-filecoin/wallet"
)

// Abstracts over a store of blockchain state.
type previewerChainReader interface {
	BlockHeight() (uint64, error)
	GetHead() types.SortedCidSet
	GetTipSetStateRoot(tsKey types.SortedCidSet) (cid.Cid, error)
}

// Previewer calculates the amount of Gas needed for a command
type Previewer struct {
	wallet *wallet.Wallet
	// To get the head tipset state root.
	chainReader previewerChainReader
	// To load the tree for the head tipset state root.
	cst *hamt.CborIpldStore
	// For vm storage.
	bs bstore.Blockstore
}

// NewPreviewer constructs a Previewer.
func NewPreviewer(wallet *wallet.Wallet, chainReader previewerChainReader, cst *hamt.CborIpldStore, bs bstore.Blockstore) *Previewer {
	return &Previewer{wallet, chainReader, cst, bs}
}

// Preview sends a read-only message to an actor.
func (p *Previewer) Preview(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) (types.GasUnits, error) {
	encodedParams, err := abi.ToEncodedValues(params...)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldnt encode message params")
	}

	st, err := chain.LatestState(ctx, p.chainReader, p.cst)
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "could load tree for latest state root")
	}
	h, err := p.chainReader.BlockHeight()
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "couldnt get base tipset height")
	}

	vms := vm.NewStorageMap(p.bs)
	usedGas, err := consensus.PreviewQueryMethod(ctx, st, vms, to, method, encodedParams, optFrom, types.NewBlockHeight(h))
	if err != nil {
		return types.NewGasUnits(0), errors.Wrap(err, "query method returned an error")
	}
	return usedGas, nil
}
