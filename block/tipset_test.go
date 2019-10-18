package block

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

const parentWeight = uint64(1337000)

var (
	cid1, cid2        cid.Cid
	mockSignerForTest types.MockSigner
	cidGetter         func() cid.Cid
)

func init() {
	cidGetter = types.NewCidForTestGetter()
	cid1 = cidGetter()
	cid2 = cidGetter()

	mockSignerForTest, _ = types.NewMockSignersAndKeyInfo(2)
}

func block(t *testing.T, ticket []byte, height int, parentCid cid.Cid, parentWeight, timestamp uint64, msg string) *Block {
	return &Block{
		Tickets:         []Ticket{{VRFProof: ticket}},
		Parents:         NewTipSetKey(parentCid),
		ParentWeight:    types.Uint64(parentWeight),
		Height:          types.Uint64(42 + uint64(height)),
		Messages:        types.TxMeta{SecpRoot: cidGetter(), BLSRoot: types.EmptyMessagesCID},
		StateRoot:       cidGetter(),
		MessageReceipts: cidGetter(),
		Timestamp:       types.Uint64(timestamp),
	}
}

func TestTipSet(t *testing.T) {
	tf.UnitTest(t)

	b1, b2, b3 := makeTestBlocks(t)

	t.Run("undefined tipset", func(t *testing.T) {
		assert.False(t, UndefTipSet.Defined())
		// No other methods are defined
	})

	t.Run("ordered by ticket", func(t *testing.T) {
		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		assert.True(t, ts.Defined())
		assert.Equal(t, b1, ts.At(0))
		assert.Equal(t, b2, ts.At(1))
		assert.Equal(t, b3, ts.At(2))
		assert.Equal(t, []*Block{b1, b2, b3}, ts.ToSlice())
	})

	t.Run("order breaks ties with CID", func(t *testing.T) {
		b1 := block(t, []byte{1}, 1, cid1, parentWeight, 1, "1")
		b2 := block(t, []byte{1}, 1, cid1, parentWeight, 2, "2")

		ts := RequireNewTipSet(t, b1, b2)
		if bytes.Compare(b1.Cid().Bytes(), b2.Cid().Bytes()) < 0 {
			assert.Equal(t, []*Block{b1, b2}, ts.ToSlice())
		} else {
			assert.Equal(t, []*Block{b2, b1}, ts.ToSlice())
		}
	})

	t.Run("len", func(t *testing.T) {
		t1 := RequireNewTipSet(t, b1)
		assert.True(t, t1.Defined())
		assert.Equal(t, 1, t1.Len())

		t3 := RequireNewTipSet(t, b1, b2, b3)
		assert.True(t, t3.Defined())
		assert.Equal(t, 3, t3.Len())
	})

	t.Run("key", func(t *testing.T) {
		assert.Equal(t, NewTipSetKey(b1.Cid()), RequireNewTipSet(t, b1).Key())
		assert.Equal(t, NewTipSetKey(b1.Cid(), b2.Cid(), b3.Cid()),
			RequireNewTipSet(t, b1, b2, b3).Key())
	})

	t.Run("height", func(t *testing.T) {
		tsHeight, _ := RequireNewTipSet(t, b1).Height()
		assert.Equal(t, uint64(b1.Height), tsHeight)
	})

	t.Run("parents", func(t *testing.T) {
		tsParents, _ := RequireNewTipSet(t, b1).Parents()
		assert.Equal(t, b1.Parents, tsParents)
	})

	t.Run("parent weight", func(t *testing.T) {
		tsParentWeight, _ := RequireNewTipSet(t, b1).ParentWeight()
		assert.Equal(t, parentWeight, tsParentWeight)
	})

	t.Run("min ticket", func(t *testing.T) {
		tsTicket, _ := RequireNewTipSet(t, b1).MinTicket()
		assert.Equal(t, b1.Tickets[0], tsTicket)

		tsTicket, _ = RequireNewTipSet(t, b2).MinTicket()
		assert.Equal(t, b2.Tickets[0], tsTicket)

		tsTicket, _ = RequireNewTipSet(t, b3, b2, b1).MinTicket()
		assert.Equal(t, b1.Tickets[0], tsTicket)
	})

	t.Run("min timestamp", func(t *testing.T) {
		tsTime, _ := RequireNewTipSet(t, b1, b2, b3).MinTimestamp()
		assert.Equal(t, b1.Timestamp, tsTime)
	})

	t.Run("equality", func(t *testing.T) {
		ts1a := RequireNewTipSet(t, b3, b2, b1)
		ts1b := RequireNewTipSet(t, b1, b2, b3)
		ts2 := RequireNewTipSet(t, b1, b2)
		ts3 := RequireNewTipSet(t, b2)

		assert.Equal(t, ts1a, ts1a)
		assert.Equal(t, ts1a, ts1b)
		assert.NotEqual(t, ts1a, ts2)
		assert.NotEqual(t, ts1a, ts3)
		assert.NotEqual(t, ts1a, UndefTipSet)
		assert.NotEqual(t, ts2, UndefTipSet)
		assert.NotEqual(t, ts3, UndefTipSet)
	})

	t.Run("slice", func(t *testing.T) {
		assert.Equal(t, []*Block{b1}, RequireNewTipSet(t, b1).ToSlice())

		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		slice := ts.ToSlice()
		assert.Equal(t, []*Block{b1, b2, b3}, slice)

		slice[1] = b1
		slice[2] = b2
		assert.NotEqual(t, slice, ts.ToSlice())
		assert.Equal(t, []*Block{b1, b2, b3}, ts.ToSlice()) // tipset is immutable
	})

	t.Run("string", func(t *testing.T) {
		// String shouldn't really need testing, but some existing code uses the string as a
		// datastore key and depends on the format exactly.
		assert.Equal(t, "{ "+b1.Cid().String()+" }", RequireNewTipSet(t, b1).String())

		expected := NewTipSetKey(b1.Cid(), b2.Cid(), b3.Cid()).String()
		assert.Equal(t, expected, RequireNewTipSet(t, b3, b2, b1).String())
	})

	t.Run("empty new tipset fails", func(t *testing.T) {
		_, err := NewTipSet()
		assert.Error(t, err)
		assert.Equal(t, errNoBlocks, err)
	})

	t.Run("duplicate block fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		ts, err := NewTipSet(b1, b2, b1)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched height fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Height = 3
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parents fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Parents = NewTipSetKey(cid1, cid2)
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched ticket arrays fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Tickets = append(b1.Tickets, b2.Tickets[0])
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parent weight fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.ParentWeight = types.Uint64(3000)
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})
}

// TestTipSetMinTicket checks that MinTicket is the minimum of the last of the
// tickets of the tipset, even when other tickets are smaller.
func TestTipSetMinTicket(t *testing.T) {
	tickets1 := []Ticket{{VRFProof: []byte{0x0}}, {VRFProof: []byte{0x3}}}
	tickets2 := []Ticket{{VRFProof: []byte{0x2}}, {VRFProof: []byte{0x4}}}
	tickets3 := []Ticket{{VRFProof: []byte{0x1}}, {VRFProof: []byte{0x5}}}
	expMinTicket := Ticket{VRFProof: []byte{0x3}}

	b1, b2, b3 := makeTestBlocks(t)
	b1.Tickets = tickets1
	b2.Tickets = tickets2
	b3.Tickets = tickets3

	ts := RequireNewTipSet(t, b1, b2, b3)
	minTicket, err := ts.MinTicket()
	assert.NoError(t, err)
	assert.Equal(t, expMinTicket, minTicket)
	assert.Equal(t, ts.At(0), b1)
	assert.Equal(t, ts.At(1), b2)
	assert.Equal(t, ts.At(2), b3)
}

func TestUndefKey(t *testing.T) {
	ts := UndefTipSet
	udKey := ts.Key()
	assert.True(t, udKey.Empty())
}

// Test methods: String, Key, ToSlice, MinTicket, Height, NewTipSet, Equals
func makeTestBlocks(t *testing.T) (*Block, *Block, *Block) {
	b1 := block(t, []byte{1}, 1, cid1, parentWeight, 1, "1")
	b2 := block(t, []byte{2}, 1, cid1, parentWeight, 2, "2")
	b3 := block(t, []byte{3}, 1, cid1, parentWeight, 3, "3")
	return b1, b2, b3
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*Block) TipSet {
	ts, err := NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}
