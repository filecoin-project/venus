package block_test

import (
	"bytes"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	blk "github.com/filecoin-project/go-filecoin/internal/pkg/block"
	e "github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	fbig "github.com/filecoin-project/specs-actors/actors/abi/big"

	tf "github.com/filecoin-project/go-filecoin/internal/pkg/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
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

func block(t *testing.T, ticket []byte, height int, parentCid cid.Cid, parentWeight, timestamp uint64, msg string) *blk.Block {
	return &blk.Block{
		Ticket:          blk.Ticket{VRFProof: ticket},
		Parents:         blk.NewTipSetKey(parentCid),
		ParentWeight:    fbig.NewInt(int64(parentWeight)),
		Height:          42 + abi.ChainEpoch(height),
		Messages:        e.NewCid(cidGetter()),
		StateRoot:       e.NewCid(cidGetter()),
		MessageReceipts: e.NewCid(cidGetter()),
		Timestamp:       timestamp,
	}
}

func TestTipSet(t *testing.T) {
	tf.UnitTest(t)

	b1, b2, b3 := makeTestBlocks(t)

	t.Run("undefined tipset", func(t *testing.T) {
		assert.False(t, blk.UndefTipSet.Defined())
		// No other methods are defined
	})

	t.Run("ordered by ticket", func(t *testing.T) {
		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		assert.True(t, ts.Defined())
		assert.Equal(t, b1, ts.At(0))
		assert.Equal(t, b2, ts.At(1))
		assert.Equal(t, b3, ts.At(2))
		assert.Equal(t, []*blk.Block{b1, b2, b3}, ts.ToSlice())
	})

	t.Run("order breaks ties with CID", func(t *testing.T) {
		b1 := block(t, []byte{1}, 1, cid1, parentWeight, 1, "1")
		b2 := block(t, []byte{1}, 1, cid1, parentWeight, 2, "2")

		ts := RequireNewTipSet(t, b1, b2)
		if bytes.Compare(b1.Cid().Bytes(), b2.Cid().Bytes()) < 0 {
			assert.Equal(t, []*blk.Block{b1, b2}, ts.ToSlice())
		} else {
			assert.Equal(t, []*blk.Block{b2, b1}, ts.ToSlice())
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
		assert.Equal(t, blk.NewTipSetKey(b1.Cid()), RequireNewTipSet(t, b1).Key())
		// sorted ticket order is b1, b2, b3
		assert.Equal(t, blk.NewTipSetKey(b1.Cid(), b2.Cid(), b3.Cid()),
			RequireNewTipSet(t, b2, b3, b1).Key())
	})

	t.Run("height", func(t *testing.T) {
		tsHeight, _ := RequireNewTipSet(t, b1).Height()
		assert.Equal(t, b1.Height, tsHeight)
	})

	t.Run("parents", func(t *testing.T) {
		tsParents, _ := RequireNewTipSet(t, b1).Parents()
		assert.Equal(t, b1.Parents, tsParents)
	})

	t.Run("parent weight", func(t *testing.T) {
		tsParentWeight, _ := RequireNewTipSet(t, b1).ParentWeight()
		assert.Equal(t, types.Uint64ToBig(parentWeight), tsParentWeight)
	})

	t.Run("min ticket", func(t *testing.T) {
		tsTicket, _ := RequireNewTipSet(t, b1).MinTicket()
		assert.Equal(t, b1.Ticket, tsTicket)

		tsTicket, _ = RequireNewTipSet(t, b2).MinTicket()
		assert.Equal(t, b2.Ticket, tsTicket)

		tsTicket, _ = RequireNewTipSet(t, b3, b2, b1).MinTicket()
		assert.Equal(t, b1.Ticket, tsTicket)
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
		assert.NotEqual(t, ts1a, blk.UndefTipSet)
		assert.NotEqual(t, ts2, blk.UndefTipSet)
		assert.NotEqual(t, ts3, blk.UndefTipSet)
	})

	t.Run("slice", func(t *testing.T) {
		assert.Equal(t, []*blk.Block{b1}, RequireNewTipSet(t, b1).ToSlice())

		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		slice := ts.ToSlice()
		assert.Equal(t, []*blk.Block{b1, b2, b3}, slice)

		slice[1] = b1
		slice[2] = b2
		assert.NotEqual(t, slice, ts.ToSlice())
		assert.Equal(t, []*blk.Block{b1, b2, b3}, ts.ToSlice()) // tipset is immutable
	})

	t.Run("string", func(t *testing.T) {
		// String shouldn't really need testing, but some existing code uses the string as a
		// datastore key and depends on the format exactly.
		assert.Equal(t, "{ "+b1.Cid().String()+" }", RequireNewTipSet(t, b1).String())

		expected := blk.NewTipSetKey(b1.Cid(), b2.Cid(), b3.Cid()).String()
		assert.Equal(t, expected, RequireNewTipSet(t, b3, b2, b1).String())
	})

	t.Run("empty new tipset fails", func(t *testing.T) {
		_, err := blk.NewTipSet()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no blocks for tipset")
	})

	t.Run("duplicate block fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		ts, err := blk.NewTipSet(b1, b2, b1)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched height fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Height = 3
		ts, err := blk.NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parents fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Parents = blk.NewTipSetKey(cid1, cid2)
		ts, err := blk.NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parent weight fails new tipset", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.ParentWeight = fbig.NewInt(3000)
		ts, err := blk.NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})
}

// TestTipSetMinTicket checks that MinTicket is the minimum of the last of the
// tickets of the tipset, even when other tickets are smaller.
func TestTipSetMinTicket(t *testing.T) {
	ticket1 := blk.Ticket{VRFProof: []byte{0x3}}
	ticket2 := blk.Ticket{VRFProof: []byte{0x4}}
	ticket3 := blk.Ticket{VRFProof: []byte{0x5}}
	expMinTicket := blk.Ticket{VRFProof: []byte{0x3}}

	b1, b2, b3 := makeTestBlocks(t)
	b1.Ticket = ticket1
	b2.Ticket = ticket2
	b3.Ticket = ticket3

	ts := RequireNewTipSet(t, b1, b2, b3)
	minTicket, err := ts.MinTicket()
	assert.NoError(t, err)
	assert.Equal(t, expMinTicket, minTicket)
	assert.Equal(t, ts.At(0), b1)
	assert.Equal(t, ts.At(1), b2)
	assert.Equal(t, ts.At(2), b3)
}

func TestUndefKey(t *testing.T) {
	ts := blk.UndefTipSet
	udKey := ts.Key()
	assert.True(t, udKey.Empty())
}

// Test methods: String, Key, ToSlice, MinTicket, Height, NewTipSet, Equals
func makeTestBlocks(t *testing.T) (*blk.Block, *blk.Block, *blk.Block) {
	b1 := block(t, []byte{1}, 1, cid1, parentWeight, 1, "1")
	b2 := block(t, []byte{2}, 1, cid1, parentWeight, 2, "2")
	b3 := block(t, []byte{3}, 1, cid1, parentWeight, 3, "3")
	return b1, b2, b3
}

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(t *testing.T, blks ...*blk.Block) blk.TipSet {
	ts, err := blk.NewTipSet(blks...)
	require.NoError(t, err)
	return ts
}
