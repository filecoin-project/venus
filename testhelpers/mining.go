package testhelpers

import (
	"crypto/rand"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"math/big"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// CreateMinerMessage creates a message to create a miner.
func CreateMinerMessage(from address.Address, nonce uint64, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (*types.Message, error) {
	params, err := abi.ToEncodedValues(big.NewInt(int64(pledge)), []byte{}, pid)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, collateral, "createMiner", params), nil
}

// AddBidMessage creates a message to add a bid.
func AddBidMessage(from address.Address, nonce uint64, price *types.AttoFIL, size *types.BytesAmount) (*types.Message, error) {
	funds := price.CalculatePrice(size)

	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, funds, "addBid", params), nil
}

// AddAskMessage creates a message to add ask.
func AddAskMessage(miner address.Address, from address.Address, nonce uint64, price *types.AttoFIL, size *types.BytesAmount) (*types.Message, error) {
	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "addAsk", params), nil
}

// CommitSectorMessage creates a message to commit a sector.
func CommitSectorMessage(miner, from address.Address, nonce, sectorID uint64, commR, commD []byte) (*types.Message, error) {
	params, err := abi.ToEncodedValues(sectorID, commR, commD)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "commitSector", params), nil
}

// MakeProof creates a random proof.
func MakeProof() []byte {
	p := makeRandomBytes(192)
	p[0] = 42
	return p
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
