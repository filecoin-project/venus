package msg

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	bstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// Abstracts over a store of blockchain state.
type previewerChainReader interface {
	GetHead() block.TipSetKey
	GetTipSetState(context.Context, block.TipSetKey) (state.Tree, error)
	GetTipSet(block.TipSetKey) (block.TipSet, error)
}

// Dragons: delete
type messagePreviewer interface {
}

// Previewer calculates the amount of Gas needed for a command
type Previewer struct {
	// To get the head tipset state root.
	chainReader previewerChainReader
	// To load the tree for the head tipset state root.
	cst cbor.IpldStore
	// For vm storage.
	bs bstore.Blockstore
	// To to preview messages
	processor messagePreviewer
}

// NewPreviewer constructs a Previewer.
func NewPreviewer(chainReader previewerChainReader, cst cbor.IpldStore, bs bstore.Blockstore, processor messagePreviewer) *Previewer {
	return &Previewer{chainReader, cst, bs, processor}
}

// Preview sends a read-only message to an actor.
func (p *Previewer) Preview(ctx context.Context, optFrom, to address.Address, method abi.MethodNum, params ...interface{}) (gas.Unit, error) {
	return gas.NewGas(0), nil
}
