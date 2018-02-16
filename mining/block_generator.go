package mining

import (
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockGenerator generates new blocks for inclusion in the chain.
type BlockGenerator struct {
	Mp *core.MessagePool
}

// Generate returns a new block created from the messages in the
// pool. It does not remove them. It can't fail.
func (b BlockGenerator) Generate(p *types.Block) *types.Block {
	child := &types.Block{
		Height:   p.Height + 1,
		Messages: b.Mp.Pending(),
	}
	child.AddParent(*p)
	return child
}
