package core

import (
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func TestTipIndex(t *testing.T) {
	assert := assert.New(t)
	idx := tipIndex{}

	block := func(height int, parentCid *cid.Cid, msg string) *types.Block {
		addrGetter := types.NewAddressForTestGetter()
		m1 := types.NewMessage(addrGetter(), addrGetter(), 0, types.NewTokenAmount(10), "hello", []byte(msg))
		m1Cid, err := m1.Cid()
		assert.NoError(err)
		return &types.Block{
			Parents:         types.NewSortedCidSet(parentCid),
			Height:          42 + uint64(height),
			Nonce:           7,
			Messages:        []*types.Message{m1},
			StateRoot:       types.SomeCid(),
			MessageReceipts: []*types.MessageReceipt{types.NewMessageReceipt(m1Cid, 1, "", []byte{1, 2})},
		}
	}

	contains := func(b *types.Block, expectedHeightEntries, expectedParentSetEntries, expectedBlocks int) {
		assert.Equal(expectedHeightEntries, len(idx))
		assert.Equal(expectedParentSetEntries, len(idx[b.Height]))
		assert.Equal(expectedBlocks, len(idx[b.Height][keyForParentSet(b.Parents)]))
		assert.True(b.Cid().Equals(idx[b.Height][keyForParentSet(b.Parents)][b.Cid().String()]))
	}

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	b1 := block(42, cid1, "foo")
	idx.addBlock(b1)
	contains(b1, 1, 1, 1)

	b2 := block(42, cid1, "bar")
	idx.addBlock(b2)
	contains(b2, 1, 1, 2)

	cid3 := cidGetter()
	b3 := block(42, cid3, "hot")
	idx.addBlock(b3)
	contains(b3, 1, 2, 1)

	cid4 := cidGetter()
	b4 := block(43, cid4, "monkey")
	idx.addBlock(b4)
	contains(b4, 2, 1, 1)
}
