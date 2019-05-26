package types

import (
	"bytes"
	"testing"

	"github.com/ipfs/go-cid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
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

func block(t *testing.T, ticket []byte, height int, parentCid cid.Cid, parentWeight uint64, msg string) *Block {
	addrGetter := address.NewForTestGetter()

	m1 := NewMessage(mockSignerForTest.Addresses[0], addrGetter(), 0, NewAttoFILFromFIL(10), "hello", []byte(msg))
	sm1, err := NewSignedMessage(*m1, &mockSignerForTest, NewGasPrice(0), NewGasUnits(0))
	require.NoError(t, err)
	ret := []byte{1, 2}

	return &Block{
		Ticket:          ticket,
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

	b1, b2, b3 := makeTestBlocks(t)

	assert.False(t, NoTipSet.Defined())

	t.Run("construction of singleton", func(t *testing.T) {
		ts := RequireNewTipSet(t, b1)
		assert.True(t, ts.Defined())
		assert.True(t, ts.IsSolo())
		assert.Equal(t, 1, ts.Len())
		assert.Equal(t, b1, ts.At(0))
		assert.Equal(t, []*Block{b1}, ts.ToSlice())

		tsParentWeight, _ := ts.ParentWeight()
		assert.Equal(t, uint64(1337000), tsParentWeight)
		assert.Equal(t, NewSortedCidSet(b1.Cid()), ts.ToSortedCidSet())
		tsParents, _ := ts.Parents()
		assert.Equal(t, b1.Parents, tsParents)
		tsHeight, _ := ts.Height()
		assert.Equal(t, uint64(b1.Height), tsHeight)
		tsTicket, _ := ts.MinTicket()
		assert.Equal(t, b1.Ticket, tsTicket)

		assert.Equal(t, "{ "+b1.Cid().String()+" }", ts.String())
	})

	t.Run("construction of multiblock", func(t *testing.T) {
		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		assert.True(t, ts.Defined())
		assert.False(t, ts.IsSolo())
		assert.Equal(t, 3, ts.Len())
		assert.Equal(t, b1, ts.At(0))
		assert.Equal(t, b2, ts.At(1))
		assert.Equal(t, b3, ts.At(2))
		assert.Equal(t, []*Block{b1, b2, b3}, ts.ToSlice())

		tsParentWeight, _ := ts.ParentWeight()
		assert.Equal(t, uint64(1337000), tsParentWeight)
		assert.Equal(t, NewSortedCidSet(b1.Cid(), b2.Cid(), b3.Cid()), ts.ToSortedCidSet())
		tsParents, _ := ts.Parents()
		assert.Equal(t, b1.Parents, tsParents)
		tsHeight, _ := ts.Height()
		assert.Equal(t, uint64(b1.Height), tsHeight)
		tsTicket, _ := ts.MinTicket()
		assert.Equal(t, b1.Ticket, tsTicket)

		assert.Equal(t, "{ "+b1.Cid().String()+" "+b2.Cid().String()+" "+b3.Cid().String()+" }", ts.String())
	})

	t.Run("break ties with CID", func(t *testing.T) {
		pq := uint64(1337000)

		b1 := block(t, []byte{1}, 1, cid1, pq, "1")
		b2 := block(t, []byte{1}, 1, cid1, pq, "2")

		ts := RequireNewTipSet(t, b1, b2)
		if bytes.Compare(b1.Cid().Bytes(), b2.Cid().Bytes()) < 0 {
			assert.Equal(t, []*Block{b1, b2}, ts.ToSlice())
		} else {
			assert.Equal(t, []*Block{b2, b1}, ts.ToSlice())
		}
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
		assert.NotEqual(t, ts1a, NoTipSet)
		assert.NotEqual(t, ts2, NoTipSet)
		assert.NotEqual(t, ts3, NoTipSet)
	})

	t.Run("slice", func(t *testing.T) {
		ts := RequireNewTipSet(t, b3, b2, b1) // Presented in reverse order
		slice := ts.ToSlice()
		assert.Equal(t, []*Block{b1, b2, b3}, slice)
		slice[1] = b1
		slice[2] = b2

		assert.NotEqual(t, slice, ts.ToSlice())
		assert.Equal(t, []*Block{b1, b2, b3}, ts.ToSlice()) // tipset is immutable
	})

	t.Run("empty", func(t *testing.T) {
		_, err := NewTipSet()
		assert.Error(t, err)
		assert.Equal(t, ErrEmptyTipSet, err)
	})

	t.Run("duplicate block", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		ts, err := NewTipSet(b1, b2, b1)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched height", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Height = 3
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parents", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.Parents = NewSortedCidSet(cid1, cid2)
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})

	t.Run("mismatched parent weight", func(t *testing.T) {
		b1, b2, b3 = makeTestBlocks(t)
		b1.ParentWeight = Uint64(3000)
		ts, err := NewTipSet(b1, b2, b3)
		assert.Error(t, err)
		assert.False(t, ts.Defined())
	})
}

// Test methods: String, ToSortedCidSet, ToSlice, MinTicket, Height, NewTipSet, Equals
func makeTestBlocks(t *testing.T) (*Block, *Block, *Block) {
	pW := uint64(1337000)

	b1 := block(t, []byte{1}, 1, cid1, pW, "1")
	b2 := block(t, []byte{2}, 1, cid1, pW, "2")
	b3 := block(t, []byte{3}, 1, cid1, pW, "3")
	return b1, b2, b3
}
