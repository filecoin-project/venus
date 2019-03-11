package testhelpers

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"testing"
	"time"

	"gx/ipfs/QmPVkJMTeRC6iBByPWdrRkD3BE5UXsj5HPzb4kPqL186mS/testify/require"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTimeTest is the block time used by workers during testing
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
	require := require.New(t)

	var tipSetsDescBlockHeight []types.TipSet
	// setup ancestor chain
	head := types.NewBlockForTest(nil, uint64(0))
	head.Ticket = []byte(strconv.Itoa(0))
	for i := 0; i < numTipSets; i++ {
		tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(require, head)}, tipSetsDescBlockHeight...)
		newBlock := types.NewBlockForTest(head, uint64(0))
		newBlock.Ticket = []byte(strconv.Itoa(i + 1))
		head = newBlock
	}

	tipSetsDescBlockHeight = append([]types.TipSet{types.RequireNewTipSet(require, head)}, tipSetsDescBlockHeight...)

	return tipSetsDescBlockHeight
}
