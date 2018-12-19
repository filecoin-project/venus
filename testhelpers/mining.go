package testhelpers

import (
	"crypto/rand"
	"github.com/filecoin-project/go-filecoin/proofs"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
	"math/big"
	"time"

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
func CommitSectorMessage(miner, from address.Address, nonce, sectorID uint64, commR, commD []byte) (*types.Message, error) {
	params, err := abi.ToEncodedValues(sectorID, commR, commD)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "commitSector", params), nil
}

// MakePoStProof creates a random proof.
func MakePoStProof() proofs.PoStProof {
	p := makeRandomBytes(192)
	p[0] = 42
	var postProof proofs.PoStProof
	for idx, elem := range p {
		postProof[idx] = elem
	}
	return postProof
}

// MakeCommitment creates a random commitment.
func MakeCommitment() []byte {
	return makeRandomBytes(32)
}

func makeRandomBytes(size int) []byte {
	comm := make([]byte, size)
	if _, err := rand.Read(comm); err != nil {
		panic(err)
	}

	return comm
}
