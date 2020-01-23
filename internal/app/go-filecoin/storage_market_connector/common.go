package storagemarketconnector

import "github.com/filecoin-project/go-filecoin/internal/pkg/block"

// Implements storagemarket.StateKey
type stateKey struct {
	ts     block.TipSetKey
	height uint64
}

func (k *stateKey) Height() uint64 {
	return k.height
}
