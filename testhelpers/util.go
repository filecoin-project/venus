package testhelpers

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	libp2ppeer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"math/big"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// GetFreePort gets a free port from the kernel
// Credit: https://github.com/phayes/freeport
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close() // nolint: errcheck
	return l.Addr().(*net.TCPAddr).Port, nil
}

func getGoPath() (string, error) {
	gp := os.Getenv("GOPATH")
	if gp != "" {
		return gp, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "go"), nil
}

//GetFilecoinBinary returns the path where the filecoin binary will be if it has been built.
func GetFilecoinBinary() (string, error) {
	gopath, err := getGoPath()
	if err != nil {
		return "", errors.Wrap(err, "failed to get GOPATH")
	}

	bin := filepath.Join(gopath, "/src/github.com/filecoin-project/go-filecoin/go-filecoin")
	_, err = os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the filecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}

// CreateMinerMessage creates a message to create a miner.
func CreateMinerMessage(from types.Address, nonce uint64, pledge types.BytesAmount, pid libp2ppeer.ID, collateral *types.AttoFIL) (*types.Message, error) {
	params, err := abi.ToEncodedValues(&pledge, []byte{}, pid)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, collateral, "createMiner", params), nil
}

// AddBidMessage creates a message to add a bid.
func AddBidMessage(from types.Address, nonce uint64, price *types.AttoFIL, size *types.BytesAmount) (*types.Message, error) {
	funds := price.CalculatePrice(size)

	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, funds, "addBid", params), nil
}

// AddAskMessage creates a message to add ask.
func AddAskMessage(miner types.Address, from types.Address, nonce uint64, price *types.AttoFIL, size *types.BytesAmount) (*types.Message, error) {
	params, err := abi.ToEncodedValues(price, size)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "addAsk", params), nil
}

// AddDealMessage a message to create a deal.
func AddDealMessage(from types.Address, nonce uint64, askID, bidID *big.Int, bidOwnerSig []byte, refb []byte) (*types.Message, error) {
	params, err := abi.ToEncodedValues(askID, bidID, bidOwnerSig, refb)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, address.StorageMarketAddress, nonce, types.NewZeroAttoFIL(), "addDeal", params), nil
}

// CommitSectorMessage creates a message to commit a sector.
func CommitSectorMessage(miner, from types.Address, nonce uint64, sectorID *big.Int, commR []byte, deals []uint64) (*types.Message, error) {
	params, err := abi.ToEncodedValues(sectorID, commR, deals)
	if err != nil {
		return nil, err
	}

	return types.NewMessage(from, miner, nonce, types.NewZeroAttoFIL(), "commitSector", params), nil
}
