package chain

import (
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/fork/blockstore"
	blockFormat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type storable interface {
	ToStorageBlock() (blockFormat.Block, error)
}

func PutMessage(bs blockstore.Blockstore, m storable) (cid.Cid, error) {
	b, err := m.ToStorageBlock()
	if err != nil {
		return cid.Undef, err
	}

	if err := bs.Put(b); err != nil {
		return cid.Undef, err
	}

	return b.Cid(), nil
}

// Reverse reverses the order of the slice `chain`.
func Reverse(chain []*block.TipSet) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(chain)/2 - 1; i >= 0; i-- {
		opp := len(chain) - 1 - i
		chain[i], chain[opp] = chain[opp], chain[i]
	}
}
