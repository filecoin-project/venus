package chain_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/chain"
	tf "github.com/filecoin-project/go-filecoin/testhelpers/testflags"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestIterAncestors(t *testing.T) {
	tf.UnitTest(t)
	miner, err := address.NewActorAddress([]byte(fmt.Sprintf("address")))
	require.NoError(t, err)

	t.Run("iterates", func(t *testing.T) {
		ctx := context.Background()
		store := chain.NewBuilder(t, miner)

		root := store.AppendBlockOnBlocks()
		b11 := store.AppendBlockOnBlocks(root)
		b12 := store.AppendBlockOnBlocks(root)
		b21 := store.AppendBlockOnBlocks(b11, b12)

		t0 := types.RequireNewTipSet(t, root)
		t1 := types.RequireNewTipSet(t, b11, b12)
		t2 := types.RequireNewTipSet(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t0.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.True(t, it.Complete())
	})

	t.Run("respects context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		store := chain.NewBuilder(t, miner)

		root := store.AppendBlockOnBlocks()
		b11 := store.AppendBlockOnBlocks(root)
		b12 := store.AppendBlockOnBlocks(root)
		b21 := store.AppendBlockOnBlocks(b11, b12)

		types.RequireNewTipSet(t, root)
		t1 := types.RequireNewTipSet(t, b11, b12)
		t2 := types.RequireNewTipSet(t, b21)

		it := chain.IterAncestors(ctx, store, t2)
		assert.False(t, it.Complete())
		assert.True(t, t2.Equals(it.Value()))

		assert.NoError(t, it.Next())
		assert.False(t, it.Complete())
		assert.True(t, t1.Equals(it.Value()))

		cancel()

		assert.Error(t, it.Next())
	})
}

func TestIterTickets(t *testing.T) {
	tf.UnitTest(t)
	miner, err := address.NewActorAddress([]byte(fmt.Sprintf("address")))
	require.NoError(t, err)

	t.Run("chain of single block single ticket tipsets", func(t *testing.T) {
		ctx := context.Background()
		builder := chain.NewBuilder(t, miner)
		height := 5
		gen := builder.NewGenesis()
		head := builder.AppendManyOn(height, gen)
		it := chain.IterTickets(ctx, builder, head)

		expected := []uint64{5, 4, 3, 2, 1, 0}
		iterExpect(t, it, expected)
	})

	t.Run("multi-block tipsets", func(t *testing.T) {
		ctx := context.Background()
		builder := chain.NewBuilder(t, miner)
		height := 2
		gen := builder.NewGenesis()             // ticket 0
		h1 := builder.AppendManyOn(height, gen) // tickets 1, 2
		h2 := builder.AppendOn(h1, 3)           // tickets 3, 4, 5
		h3 := builder.AppendOn(h2, 2)           // tickets 6, 7
		it := chain.IterTickets(ctx, builder, h3)

		expected := []uint64{6, 3, 2, 1, 0}
		iterExpect(t, it, expected)
	})

	t.Run("chain with null blocks", func(t *testing.T) {
		ctx := context.Background()
		builder := chain.NewBuilder(t, miner)
		gen := builder.NewGenesis() // ticket 0
		ticketsA := [][]byte{
			uint64ToByte(6),
			uint64ToByte(72),
			uint64ToByte(10),
		}
		ticketsB := [][]byte{
			uint64ToByte(5),
			uint64ToByte(6),
			uint64ToByte(34),
		}
		ticketsC := [][]byte{
			uint64ToByte(4),
			uint64ToByte(6),
			uint64ToByte(35),
		}
		tipSetTickets := [][][]byte{ticketsA, ticketsB, ticketsC}

		h1 := builder.BuildOn(gen, 3, func(bb *chain.BlockBuilder, i int) {
			bb.SetTickets(tipSetTickets[i]) // 33, 72, 6
		})

		h2 := builder.BuildOn(h1, 1, func(bb *chain.BlockBuilder, i int) {
			bb.SetTickets([][]byte{
				uint64ToByte(1),
				uint64ToByte(7),
				uint64ToByte(9),
			}) // 9, 7, 1
		})

		it := chain.IterTickets(ctx, builder, h2)
		expected := []uint64{9, 7, 1, 10, 72, 6, 0}
		iterExpect(t, it, expected)
	})
}

// helpers

func iterExpect(t *testing.T, it *chain.TicketIterator, expected []uint64) {
	t.Helper()
	for _, exp := range expected {
		assert.False(t, it.Complete())
		assert.Equal(t, exp, ticketToUint64(it.Value()))
		assert.NoError(t, it.Next())
	}
	assert.True(t, it.Complete())
	assert.Error(t, it.Next())

}

func ticketToUint64(ticket types.Ticket) uint64 {
	fmt.Printf("ticket %x\n", ticket.VDFResult)
	return binary.BigEndian.Uint64(ticket.VDFResult)
}

func uint64ToByte(i uint64) []byte {
	out := make([]byte, binary.Size(i))
	binary.BigEndian.PutUint64(out, i)
	return out
}
