package core

import (
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"sort"
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
	"github.com/stretchr/testify/assert"
)

func block(assert *assert.Assertions, height int, parentCid *cid.Cid, msg string) *types.Block {
	addrGetter := types.NewAddressForTestGetter()
	m1 := types.NewMessage(addrGetter(), addrGetter(), 0, types.NewTokenAmount(10), "hello", []byte(msg))
	ret := []byte{1, 2}

	return &types.Block{
		Parents:         types.NewSortedCidSet(parentCid),
		Height:          42 + uint64(height),
		Nonce:           7,
		Messages:        []*types.Message{m1},
		StateRoot:       types.SomeCid(),
		MessageReceipts: []*types.MessageReceipt{{ExitCode: 1, Return: []types.Bytes{ret}}},
	}
}

func TestTipSet(t *testing.T) {
	assert := assert.New(t)

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()

	b1 := block(assert, 1, cid1, "1")
	b2 := block(assert, 1, cid1, "2")
	b3 := block(assert, 1, cid1, "3")

	ts := TipSet{}
	ts[b1.Cid().String()] = b1

	ts2 := ts.Clone()
	assert.Equal(ts2, ts) // note: assert.Equal() does a deep comparison, not same as Golang == operator
	assert.False(&ts2 == &ts)

	ts[b2.Cid().String()] = b2
	assert.NotEqual(ts2, ts)
	assert.Equal(2, len(ts))
	assert.Equal(1, len(ts2))

	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	ts2[b1.Cid().String()] = b3
	assert.NotEqual(ts2, ts)
	assert.Equal([]byte("3"), ts2[b1.Cid().String()].Messages[0].Params)
	assert.Equal([]byte("1"), ts[b1.Cid().String()].Messages[0].Params)

	// The actual values inside the TipSets are not copied - we assume they are used immutably.
	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	oldB1 := ts[b1.Cid().String()]
	ts[oldB1.Cid().String()].Nonce = 17
	assert.Equal(ts2, ts)
}

// Test methods: String, ToSortedCidSet, ToSlice, Score, MinTicket, Height, NewTipSet, Equals
func TestTipSetMethods(t *testing.T) {
	assert := assert.New(t)

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()

	b1 := block(assert, 1, cid1, "1")
	b1.Ticket = []byte{0}
	b2 := block(assert, 1, cid1, "2")
	b2.Ticket = []byte{1}
	b3 := block(assert, 1, cid1, "3")
	b3.Ticket = []byte{0}

	// NewTipSet
	tips := []*types.Block{b1, b2, b3}
	ts := NewTipSet(tips...)
	assert.Equal(ts[b1.Cid().String()], b1)
	assert.Equal(ts[b2.Cid().String()], b2)
	assert.Equal(ts[b3.Cid().String()], b3)
	assert.Equal(3, len(ts))

	// MinTicket
	mt := ts.MinTicket()
	assert.Equal(types.Signature([]byte{0}), mt)

	// Height
	h := ts.Height()
	assert.Equal(uint64(43), h)

	// Score
	sc := ts.Score()
	assert.Equal(uint64(43*103), sc)

	// ToSortedCidSet
	cidsExp := types.NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	strExp := cidsExp.String()
	assert.Equal(strExp, ts.String())
	assert.Equal(cidsExp, ts.ToSortedCidSet())

	// ToSlice
	blks := ts.ToSlice()
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].Cid().String() < tips[j].Cid().String()
	})
	sort.Slice(blks, func(i, j int) bool {
		return blks[i].Cid().String() < blks[j].Cid().String()
	})
	assert.Equal(tips, blks)

	// Equals & AddBlock
	ts2 := NewTipSet(b1, b2)
	assert.True(!ts2.Equals(ts))
	ts2.AddBlock(b3)
	assert.True(ts.Equals(ts2))
}

func TestTipIndex(t *testing.T) {
	assert := assert.New(t)
	idx := tipIndex{}

	contains := func(b *types.Block, expectedHeightEntries, expectedParentSetEntries, expectedBlocks int) {
		assert.Equal(expectedHeightEntries, len(idx))
		assert.Equal(expectedParentSetEntries, len(idx[b.Height]))
		assert.Equal(expectedBlocks, len(idx[b.Height][keyForParentSet(b.Parents)]))
		assert.True(b.Cid().Equals(idx[b.Height][keyForParentSet(b.Parents)][b.Cid().String()].Cid()))
	}

	cidGetter := types.NewCidForTestGetter()
	cid1 := cidGetter()
	b1 := block(assert, 42, cid1, "foo")
	idx.addBlock(b1)
	contains(b1, 1, 1, 1)

	b2 := block(assert, 42, cid1, "bar")
	idx.addBlock(b2)
	contains(b2, 1, 1, 2)

	cid3 := cidGetter()
	b3 := block(assert, 42, cid3, "hot")
	idx.addBlock(b3)
	contains(b3, 1, 2, 1)

	cid4 := cidGetter()
	b4 := block(assert, 43, cid4, "monkey")
	idx.addBlock(b4)
	contains(b4, 2, 1, 1)
}
