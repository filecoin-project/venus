package mining

import (
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockGenerator generates new blocks for inclusion in the chain.
type BlockGenerator struct {
	Mp *core.MessagePool
}

// Generate returns a new block created from the messages in the
// pool. It does not remove them. It can't fail.
func (b BlockGenerator) Generate(cid *cid.Cid, h uint64) *types.Block {
	return &types.Block{
		Parent:   cid,
		Height:   h + 1,
		Messages: b.Mp.Pending(),
	}
}
