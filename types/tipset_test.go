package types

import (
	"context"
	"sort"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	cid1, cid2        cid.Cid
	mockSignerForTest MockSigner
)

func init() {
	cidGetter := NewCidForTestGetter()
	cid1 = cidGetter()
	cid2 = cidGetter()

	mockSignerForTest, _ = NewMockSignersAndKeyInfo(2)
}

// requireTipSetAdd adds a block to the provided tipset and requires that this
// does not error.
func requireTipSetAdd(require *require.Assertions, blk *Block, ts TipSet) {
	err := ts.AddBlock(blk)
	require.NoError(err)
}

func block(require *require.Assertions, height int, parentCid cid.Cid, parentWeight uint64, msg string) *Block {
	addrGetter := address.NewForTestGetter()

	m1 := NewMessage(mockSignerForTest.Addresses[0], addrGetter(), 0, NewAttoFILFromFIL(10), "hello", []byte(msg))
	sm1, err := NewSignedMessage(*m1, &mockSignerForTest, NewGasPrice(0), NewGasUnits(0))
	require.NoError(err)
	ret := []byte{1, 2}

	return &Block{
		Parents:         NewSortedCidSet(parentCid),
		ParentWeight:    Uint64(parentWeight),
		Height:          Uint64(42 + uint64(height)),
		Nonce:           7,
		Messages:        []*SignedMessage{sm1},
		StateRoot:       SomeCid(),
		MessageReceipts: []*MessageReceipt{{ExitCode: 1, Return: [][]byte{ret}}},
	}
}

func TestTipSet(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	b1 := block(require, 1, cid1, uint64(1137), "1")
	b2 := block(require, 1, cid1, uint64(1137), "2")
	b3 := block(require, 1, cid1, uint64(1137), "3")

	ts := TipSet{}
	ts[b1.Cid()] = b1

	ts2 := ts.Clone()
	assert.Equal(ts2, ts) // note: assert.Equal() does a deep comparison, not same as Golang == operator
	assert.False(&ts2 == &ts)

	ts[b2.Cid()] = b2
	assert.NotEqual(ts2, ts)
	assert.Equal(2, len(ts))
	assert.Equal(1, len(ts2))

	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	ts2[b1.Cid()] = b3
	assert.NotEqual(ts2, ts)
	assert.Equal([]byte("3"), ts2[b1.Cid()].Messages[0].Params)
	assert.Equal([]byte("1"), ts[b1.Cid()].Messages[0].Params)

	// The actual values inside the TipSets are not copied - we assume they are used immutably.
	ts2 = ts.Clone()
	assert.Equal(ts2, ts)
	oldB1 := ts[b1.Cid()]
	ts[oldB1.Cid()].Nonce = 17
	assert.Equal(ts2, ts)
}

type testBlockGetter struct {
	block       *Block
	expectedCid cid.Cid
	require     *require.Assertions
}

func (t *testBlockGetter) GetBlock(ctx context.Context, id cid.Cid) (*Block, error) {
	t.require.Equal(t.expectedCid, id)
	return t.block, nil
}

func TestGetNext(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	ctx := context.Background()
	b1, b2, b3 := RequireTestBlocks(t)
	ts := RequireNewTipSet(require, b1, b2)

	fakeBlockStore := &testBlockGetter{
		block:       b3,
		expectedCid: cid1,
		require:     require,
	}

	expectedResult := &TipSet{}
	err := expectedResult.AddBlock(b3)
	require.NoError(err)

	result, err := ts.GetNext(ctx, fakeBlockStore)
	require.NoError(err)
	assert.Equal(expectedResult, result)
}

// Test methods: String, ToSortedCidSet, ToSlice, MinTicket, Height, NewTipSet, Equals
func RequireTestBlocks(t *testing.T) (*Block, *Block, *Block) {
	require := require.New(t)

	pW := uint64(1337000)

	b1 := block(require, 1, cid1, pW, "1")
	b1.Ticket = []byte{0}
	b2 := block(require, 1, cid1, pW, "2")
	b2.Ticket = []byte{1}
	b3 := block(require, 1, cid1, pW, "3")
	b3.Ticket = []byte{0}
	return b1, b2, b3
}

func RequireTestTipSet(t *testing.T) TipSet {
	require := require.New(t)
	b1, b2, b3 := RequireTestBlocks(t)
	return RequireNewTipSet(require, b1, b2, b3)
}

func TestTipSetAddBlock(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	b1, b2, b3 := RequireTestBlocks(t)

	// Add Valid
	ts1 := TipSet{}
	requireTipSetAdd(require, b1, ts1)
	assert.Equal(1, len(ts1))
	requireTipSetAdd(require, b2, ts1)
	requireTipSetAdd(require, b3, ts1)

	ts2 := RequireNewTipSet(require, b1, b2, b3)
	assert.Equal(ts2, ts1)

	// Invalid height
	b2.Height = 5
	ts := TipSet{}
	requireTipSetAdd(require, b1, ts)
	err := ts.AddBlock(b2)
	assert.Error(err)
	b2.Height = b1.Height

	// Invalid parent set
	b2.Parents = NewSortedCidSet(cid1, cid2)
	ts = TipSet{}
	requireTipSetAdd(require, b1, ts)
	err = ts.AddBlock(b2)
	assert.Error(err)
	b2.Parents = b1.Parents

	// Invalid weight
	b2.ParentWeight = Uint64(3000)
	ts = TipSet{}
	requireTipSetAdd(require, b1, ts)
	err = ts.AddBlock(b2)
	assert.Error(err)
}

func TestNewTipSet(t *testing.T) {
	assert := assert.New(t)
	b1, b2, b3 := RequireTestBlocks(t)

	// Valid blocks
	ts, err := NewTipSet(b1, b2, b3)
	assert.NoError(err)
	assert.Equal(ts[b1.Cid()], b1)
	assert.Equal(ts[b2.Cid()], b2)
	assert.Equal(ts[b3.Cid()], b3)
	assert.Equal(3, len(ts))

	// Invalid heights
	b1, b2, b3 = RequireTestBlocks(t)
	b1.Height = 3
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(err)
	assert.Nil(ts)
	b1.Height = b2.Height

	// Invalid parent sets
	b1, b2, b3 = RequireTestBlocks(t)
	b1.Parents = NewSortedCidSet(cid1, cid2)
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(err)
	assert.Nil(ts)
	b1.Parents = b2.Parents

	// Invalid parent weights
	b1, b2, b3 = RequireTestBlocks(t)
	b1.ParentWeight = Uint64(3000)
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(err)
	assert.Nil(ts)
}

func TestTipSetMinTicket(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	mt, err := ts.MinTicket()
	assert.NoError(err)
	assert.Equal(Signature([]byte{0}), mt)
}

func TestTipSetHeight(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	h, err := ts.Height()
	assert.NoError(err)
	assert.Equal(uint64(43), h)
}

func TestTipSetParents(t *testing.T) {
	assert := assert.New(t)
	b1, _, _ := RequireTestBlocks(t)
	ts := RequireTestTipSet(t)
	ps, err := ts.Parents()
	assert.NoError(err)
	assert.Equal(ps, b1.Parents)
}

func TestTipSetParentWeight(t *testing.T) {
	assert := assert.New(t)
	ts := RequireTestTipSet(t)
	w, err := ts.ParentWeight()
	assert.NoError(err)
	assert.Equal(w, uint64(1337000))
}

func TestTipSetToSortedCidSet(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)

	cidsExp := NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	assert.Equal(cidsExp, ts.ToSortedCidSet())
}

func TestTipSetString(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)

	cidsExp := NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	strExp := cidsExp.String()
	assert.Equal(strExp, ts.String())
}

func TestTipSetToSlice(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	tips := []*Block{b1, b2, b3}
	assert := assert.New(t)

	blks := ts.ToSlice()
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].Cid().String() < tips[j].Cid().String()
	})
	sort.Slice(blks, func(i, j int) bool {
		return blks[i].Cid().String() < blks[j].Cid().String()
	})
	assert.Equal(tips, blks)

	assert.Equal(ts.ToSlice(), ts.ToSlice())

	tsEmpty := TipSet{}
	slEmpty := tsEmpty.ToSlice()
	assert.Equal(0, len(slEmpty))
}

func TestTipSetEquals(t *testing.T) {
	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	assert := assert.New(t)
	require := require.New(t)

	ts2 := RequireNewTipSet(require, b1, b2)
	assert.True(!ts2.Equals(ts))
	assert.NoError(ts2.AddBlock(b3))
	assert.True(ts.Equals(ts2))
}

/*func TestTipIndex(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	idx := tipIndex{}

	contains := func(b *Block, expectedHeightEntries, expectedParentSetEntries, expectedBlocks int) {
		assert.Equal(expectedHeightEntries, len(idx))
		assert.Equal(expectedParentSetEntries, len(idx[uint64(b.Height)]))
		assert.Equal(expectedBlocks, len(idx[uint64(b.Height)][KeyForParentSet(b.Parents)]))
		assert.True(b.Cid().Equals(idx[uint64(b.Height)][KeyForParentSet(b.Parents)][b.Cid().String()].Cid()))
	}

	cidGetter := NewCidForTestGetter()
	cid1 := cidGetter()
	b1 := block(require, 42, cid1, uint64(1137), "foo")
	idx.addBlock(b1)
	contains(b1, 1, 1, 1)

	b2 := block(require, 42, cid1, uint64(1137), "bar")
	idx.addBlock(b2)
	contains(b2, 1, 1, 2)

	cid3 := cidGetter()
	b3 := block(require, 42, cid3, uint64(1137), "hot")
	idx.addBlock(b3)
	contains(b3, 1, 2, 1)

	cid4 := cidGetter()
	b4 := block(require, 43, cid4, uint64(1137), "monkey")
	idx.addBlock(b4)
	contains(b4, 2, 1, 1)
}*/
