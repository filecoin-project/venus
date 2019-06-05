package testhelpers

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTimeTest is the block time used by workers during testing
const BlockTimeTest = time.Second

// TestBlockClock implement plumbing/clock.BlockClock
type TestBlockClock struct {
	blockTime time.Duration
}

// NewTestBlockClock returns a TestBlockClock that uses BlockTimeTest
// as its duration
func NewTestBlockClock() *TestBlockClock {
	return &TestBlockClock{
		blockTime: BlockTimeTest,
	}
}

// BlockTime returns the block time of TestBlockClock
func (tbc *TestBlockClock) BlockTime(ctx context.Context) time.Duration {
	return tbc.blockTime
}

// MakeCommitment creates a random commitment.
func MakeCommitment() []byte {
	return MakeRandomBytes(32)
}

// MakeRandomBytes generates a randomized byte slice of size 'size'
func MakeRandomBytes(size int) []byte {
	comm := make([]byte, size)
	if _, err := rand.Read(comm); err != nil {
		panic(err)
	}

	return comm
}

// RequireTipSetChain produces a chain of TipSet of the requested length. The
// TipSet with greatest height will be at the front of the returned slice.
func RequireTipSetChain(t *testing.T, numTipSets int) []types.TipSet {
	var tipSetsDescBlockHeight []types.TipSet
	// setup ancestor chain
	head := types.NewBlockForTest(nil, uint64(0))
	head.Ticket = []byte(strconv.Itoa(0))
	for i := 0; i < numTipSets; i++ {
		tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(t, head)}, tipSetsDescBlockHeight...)
		newBlock := types.NewBlockForTest(head, uint64(0))
		newBlock.Ticket = []byte(strconv.Itoa(i + 1))
		head = newBlock
	}

	tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(t, head)}, tipSetsDescBlockHeight...)

	return tipSetsDescBlockHeight
}
