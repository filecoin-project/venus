package testhelpers

import (
	"crypto/rand"
	"github.com/filecoin-project/go-filecoin/types"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	"math/big"
	"time"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
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
func CommitSectorMessage(miner, from address.Address, nonce, sectorID uint64, commR, commD []byte) (*types.Message, error) {
	params, err := abi.ToEncodedValues(sectorID, commR, commD)
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
