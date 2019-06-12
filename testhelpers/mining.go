package testhelpers

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// CreateMinerMessage creates a message to create a miner.
func CreateMinerMessage(from address.Address, nonce uint64, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (*types.Message, error) {
	params, err := abi.ToEncodedValues(big.NewInt(int64(pledge)), []byte{}, pid)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, collateral, "createMiner", params), nil
}

// CommitSectorMessage creates a message to commit a sector.
func CommitSectorMessage(miner, from address.Address, nonce, sectorID uint64, commD, commR, commRStar, proof []byte) (*types.Message, error) {
	params, err := abi.ToEncodedValues(sectorID, commD, commR, commRStar, proof)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "commitSector", params), nil
}

// TestWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type TestWorkerPorcelainAPI struct {
	blockTime time.Duration
}

// NewDefaultTestWorkerPorcelainAPI returns a TestWrokerPorcelainAPI.
func NewDefaultTestWorkerPorcelainAPI() *TestWorkerPorcelainAPI {
	return &TestWorkerPorcelainAPI{blockTime: BlockTimeTest}
}

// BlockTime returns the blocktime TestWrokerPorcelainAPI si configured with.
func (t *TestWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
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
