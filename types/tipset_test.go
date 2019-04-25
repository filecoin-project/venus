package types

import (
	"sort"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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
func requireTipSetAdd(t *testing.T, blk *Block, ts TipSet) {
	err := ts.AddBlock(blk)
	require.NoError(t, err)
}

func block(t *testing.T, height int, parentCid cid.Cid, parentWeight uint64, msg string) *Block {
	addrGetter := address.NewForTestGetter()

	m1 := NewMessage(mockSignerForTest.Addresses[0], addrGetter(), 0, NewAttoFILFromFIL(10), "hello", []byte(msg))
	sm1, err := NewSignedMessage(*m1, &mockSignerForTest, NewGasPrice(0), NewGasUnits(0))
	require.NoError(t, err)
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
	tf.UnitTest(t)

	b1 := block(t, 1, cid1, uint64(1137), "1")
	b2 := block(t, 1, cid1, uint64(1137), "2")
	b3 := block(t, 1, cid1, uint64(1137), "3")

	ts := TipSet{}
	ts[b1.Cid()] = b1

	ts2 := ts.Clone()
	assert.Equal(t, ts2, ts) // note: assert.Equal() does a deep comparison, not same as Golang == operator
	assert.False(t, &ts2 == &ts)

	ts[b2.Cid()] = b2
	assert.NotEqual(t, ts2, ts)
	assert.Equal(t, 2, len(ts))
	assert.Equal(t, 1, len(ts2))

	ts2 = ts.Clone()
	assert.Equal(t, ts2, ts)
	ts2[b1.Cid()] = b3
	assert.NotEqual(t, ts2, ts)
	assert.Equal(t, []byte("3"), ts2[b1.Cid()].Messages[0].Params)
	assert.Equal(t, []byte("1"), ts[b1.Cid()].Messages[0].Params)

	// The actual values inside the TipSets are not copied - we assume they are used immutably.
	ts2 = ts.Clone()
	assert.Equal(t, ts2, ts)
	oldB1 := ts[b1.Cid()]
	ts[oldB1.Cid()].Nonce = 17
	assert.Equal(t, ts2, ts)
}

// Test methods: String, ToSortedCidSet, ToSlice, MinTicket, Height, NewTipSet, Equals
func RequireTestBlocks(t *testing.T) (*Block, *Block, *Block) {
	pW := uint64(1337000)

	b1 := block(t, 1, cid1, pW, "1")
	b1.Ticket = []byte{0}
	b2 := block(t, 1, cid1, pW, "2")
	b2.Ticket = []byte{1}
	b3 := block(t, 1, cid1, pW, "3")
	b3.Ticket = []byte{0}
	return b1, b2, b3
}

func RequireTestTipSet(t *testing.T) TipSet {
	b1, b2, b3 := RequireTestBlocks(t)
	return RequireNewTipSet(t, b1, b2, b3)
}

func TestTipSetAddBlock(t *testing.T) {
	tf.UnitTest(t)

	b1, b2, b3 := RequireTestBlocks(t)

	// Add Valid
	ts1 := TipSet{}
	requireTipSetAdd(t, b1, ts1)
	assert.Equal(t, 1, len(ts1))
	requireTipSetAdd(t, b2, ts1)
	requireTipSetAdd(t, b3, ts1)

	ts2 := RequireNewTipSet(t, b1, b2, b3)
	assert.Equal(t, ts2, ts1)

	// Invalid height
	b2.Height = 5
	ts := TipSet{}
	requireTipSetAdd(t, b1, ts)
	err := ts.AddBlock(b2)
	assert.Error(t, err)
	b2.Height = b1.Height

	// Invalid parent set
	b2.Parents = NewSortedCidSet(cid1, cid2)
	ts = TipSet{}
	requireTipSetAdd(t, b1, ts)
	err = ts.AddBlock(b2)
	assert.Error(t, err)
	b2.Parents = b1.Parents

	// Invalid weight
	b2.ParentWeight = Uint64(3000)
	ts = TipSet{}
	requireTipSetAdd(t, b1, ts)
	err = ts.AddBlock(b2)
	assert.Error(t, err)
}

func TestNewTipSet(t *testing.T) {
	tf.UnitTest(t)

	b1, b2, b3 := RequireTestBlocks(t)

	// Valid blocks
	ts, err := NewTipSet(b1, b2, b3)
	assert.NoError(t, err)
	assert.Equal(t, ts[b1.Cid()], b1)
	assert.Equal(t, ts[b2.Cid()], b2)
	assert.Equal(t, ts[b3.Cid()], b3)
	assert.Equal(t, 3, len(ts))

	// Invalid heights
	b1, b2, b3 = RequireTestBlocks(t)
	b1.Height = 3
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(t, err)
	assert.Nil(t, ts)
	b1.Height = b2.Height

	// Invalid parent sets
	b1, b2, b3 = RequireTestBlocks(t)
	b1.Parents = NewSortedCidSet(cid1, cid2)
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(t, err)
	assert.Nil(t, ts)
	b1.Parents = b2.Parents

	// Invalid parent weights
	b1, b2, b3 = RequireTestBlocks(t)
	b1.ParentWeight = Uint64(3000)
	ts, err = NewTipSet(b1, b2, b3)
	assert.Error(t, err)
	assert.Nil(t, ts)
}

func TestTipSetMinTicket(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	mt, err := ts.MinTicket()
	assert.NoError(t, err)
	assert.Equal(t, Signature([]byte{0}), mt)
}

func TestTipSetHeight(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	h, err := ts.Height()
	assert.NoError(t, err)
	assert.Equal(t, uint64(43), h)
}

func TestTipSetParents(t *testing.T) {
	tf.UnitTest(t)

	b1, _, _ := RequireTestBlocks(t)
	ts := RequireTestTipSet(t)
	ps, err := ts.Parents()
	assert.NoError(t, err)
	assert.Equal(t, ps, b1.Parents)
}

func TestTipSetParentWeight(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	w, err := ts.ParentWeight()
	assert.NoError(t, err)
	assert.Equal(t, w, uint64(1337000))
}

func TestTipSetToSortedCidSet(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)

	cidsExp := NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	assert.Equal(t, cidsExp, ts.ToSortedCidSet())
}

func TestTipSetString(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)

	cidsExp := NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid())
	strExp := cidsExp.String()
	assert.Equal(t, strExp, ts.String())
}

func TestTipSetToSlice(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)
	tips := []*Block{b1, b2, b3}

	blks := ts.ToSlice()
	sort.Slice(tips, func(i, j int) bool {
		return tips[i].Cid().String() < tips[j].Cid().String()
	})
	sort.Slice(blks, func(i, j int) bool {
		return blks[i].Cid().String() < blks[j].Cid().String()
	})
	assert.Equal(t, tips, blks)

	assert.Equal(t, ts.ToSlice(), ts.ToSlice())

	tsEmpty := TipSet{}
	slEmpty := tsEmpty.ToSlice()
	assert.Equal(t, 0, len(slEmpty))
}

func TestTipSetEquals(t *testing.T) {
	tf.UnitTest(t)

	ts := RequireTestTipSet(t)
	b1, b2, b3 := RequireTestBlocks(t)

	ts2 := RequireNewTipSet(t, b1, b2)
	assert.True(t, !ts2.Equals(ts))
	assert.NoError(t, ts2.AddBlock(b3))
	assert.True(t, ts.Equals(ts2))
}
