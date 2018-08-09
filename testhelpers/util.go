package testhelpers

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"
	libp2ppeer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"math/big"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/crypto"
	cu "github.com/filecoin-project/go-filecoin/crypto/util"
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

const (
	// SECP256K1 is a curve used to compute private keys
	SECP256K1 = "secp256k1"
)

// WalletFile holds the values representing a wallet when writing to a file
type WalletFile struct {
	AddressKeyPairs []AddressKeyInfo `toml:"wallet"`
}

// AddressKeyInfo holds the address and KeyInfo used to generate it
type AddressKeyInfo struct {
	AddressInfo AddressInfo   `toml:"addressinfo"`
	KeyInfo     types.KeyInfo `toml:"keyinfo"`
}

// AddressInfo holds an address and a balance
type AddressInfo struct {
	Address string `toml:"address"`
	Balance uint64 `toml:"balance"`
}

// TypesAddressInfo makes it easier for the receiever to deal with types already made
type TypesAddressInfo struct {
	Address types.Address
	Balance *types.AttoFIL
}

// LoadWalletAddressAndKeysFromFile will load the addresses and their keys from File
// `file`. The balance field may also be set to specify what an address's balance
// should be at time of genesis.
func LoadWalletAddressAndKeysFromFile(file string) (map[TypesAddressInfo]types.KeyInfo, error) {
	wf, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read wallet file")
	}

	wt := new(WalletFile)
	if err = toml.Unmarshal(wf, &wt); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal wallet file")
	}

	if len(wt.AddressKeyPairs) == 0 {
		return nil, errors.New("wallet file does not contain any addresses")
	}

	loadedAddrs := make(map[TypesAddressInfo]types.KeyInfo)
	for _, akp := range wt.AddressKeyPairs {
		addr, err := types.NewAddressFromString(akp.AddressInfo.Address)
		if err != nil {
			return nil, err
		}
		tai := TypesAddressInfo{
			Address: addr,
			Balance: types.NewAttoFILFromFIL(akp.AddressInfo.Balance),
		}
		loadedAddrs[tai] = akp.KeyInfo
	}
	return loadedAddrs, nil
}

// GenerateWalletFile will generate a WalletFile with `numAddrs` addresses
func GenerateWalletFile(numAddrs int) (*WalletFile, error) {
	var wt WalletFile
	for i := 0; i < numAddrs; i++ {
		prv, err := crypto.GenerateKey()
		if err != nil {
			return nil, err
		}

		pub, ok := prv.Public().(*ecdsa.PublicKey)
		if !ok {
			// means a something is wrong with key generation
			panic("unknown public key type")
		}

		addrHash, err := types.AddressHash(cu.SerializeUncompressed(pub))
		if err != nil {
			return nil, err
		}
		// TODO: Use the address type we are running on from the config.
		newAddr := types.NewMainnetAddress(addrHash)

		ki := &types.KeyInfo{
			PrivateKey: crypto.ECDSAToBytes(prv),
			Curve:      SECP256K1,
		}

		aki := AddressKeyInfo{
			AddressInfo: AddressInfo{
				Address: newAddr.String(),
				Balance: 10000000,
			},
			KeyInfo: *ki,
		}
		wt.AddressKeyPairs = append(wt.AddressKeyPairs, aki)
	}
	return &wt, nil
}

// WriteWalletFile will write WalletFile `wf` to path `file`
func WriteWalletFile(file string, wf WalletFile) error {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to open wallet file for writing")
	}
	defer f.Close() // nolint: errcheck

	if err := toml.NewEncoder(f).Encode(wf); err != nil {
		return errors.Wrap(err, "faild to encode wallet")
	}
	return nil
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
