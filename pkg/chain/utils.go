package chain

import (
	"github.com/filecoin-project/venus/pkg/fork/blockstore"
	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
)

type storable interface {
	ToStorageBlock() (block.Block, error)
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
