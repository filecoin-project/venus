package state

import (
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/pkg/block"
)

// Abstracts over a store of blockchain state.
type chainStateChainReader interface {
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
	GetTipSetStateRoot(key block.TipSetKey) (cid.Cid, error)
	GenesisRootCid() cid.Cid
}

// TipSetStateViewer loads state views for tipsets.
type TipSetStateViewer struct {
	// To get the head tipset state root.
	chainReader chainStateChainReader
	// To load the tree for the head tipset state root.
	cst cbor.IpldStore
}

// NewTipSetStateViewer constructs a TipSetStateViewer.
func NewTipSetStateViewer(chainReader chainStateChainReader, cst cbor.IpldStore) *TipSetStateViewer {
	return &TipSetStateViewer{chainReader, cst}
}

// StateView creates a state view after the application of a tipset's messages.
func (cs TipSetStateViewer) StateView(baseKey block.TipSetKey) (*View, error) {
	root, err := cs.chainReader.GetTipSetStateRoot(baseKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get state root for %s", baseKey.String())
	}

	return NewView(cs.cst, root), nil
}
