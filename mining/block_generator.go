package mining

import (
	"context"
	"math/big"

	hamt "gx/ipfs/QmZhoiN2zi5SBBBKb181dQm4QdvWAvEwbppZvKpp4gRyNY/go-hamt-ipld"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockGenerator is the primary interface for blockGenerator.
type BlockGenerator interface {
	Generate(context.Context, *types.Block) (*types.Block, error)
}

// NewBlockGenerator returns a new BlockGenerator.
func NewBlockGenerator(mp *core.MessagePool, cst *hamt.CborIpldStore, proc core.Processor, maddr types.Address) BlockGenerator {
	return &blockGenerator{
		Mp:  mp,
		Cst: cst, Processor: proc,
		MinerAddr: maddr,
	}
}

// blockGenerator generates new blocks for inclusion in the chain.
type blockGenerator struct {
	Mp        *core.MessagePool
	Cst       *hamt.CborIpldStore
	Processor core.Processor
	MinerAddr types.Address
}

// TODO: this needs a better home
const filecoinNetworkAddr = types.Address("filecoin")

var miningReward = big.NewInt(1000)

// Generate returns a new block created from the messages in the
// pool. It does not remove them.
func (b blockGenerator) Generate(ctx context.Context, p *types.Block) (*types.Block, error) {
	st, err := types.LoadStateTree(ctx, b.Cst, p.StateRoot)
	if err != nil {
		return nil, err
	}

	// TODO: this could be passed in as a "getRewardMsg" functor
	reward := types.NewMessage(filecoinNetworkAddr, b.MinerAddr, miningReward, "", nil)

	child := &types.Block{
		Height:   p.Height + 1,
		Messages: append([]*types.Message{reward}, b.Mp.Pending()...),
	}

	child.AddParent(p)

	if err := b.Processor(ctx, child, st); err != nil {
		return nil, err
	}
	newStCid, err := st.Flush(ctx)
	if err != nil {
		return nil, err
	}
	child.StateRoot = newStCid

	return child, nil
}
