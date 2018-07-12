package testhelpers

import (
	"crypto/ecdsa"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmWHbPAp5UWfwZE3XCgD93xsCYZyk12tAAQVL3QXLKcWaj/toml"

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

//GetFilecoinBinary returns the path where the filecoin binary will be if it has been built.
func GetFilecoinBinary() (string, error) {
	bin := filepath.FromSlash(fmt.Sprintf("%s/src/github.com/filecoin-project/go-filecoin/go-filecoin", os.Getenv("GOPATH")))
	_, err := os.Stat(bin)
	if err == nil {
		return bin, nil
	}

	if os.IsNotExist(err) {
		return "", fmt.Errorf("You are missing the filecoin binary...try building, searched in '%s'", bin)
	}

	return "", err
}

const (
	// SECP256K1 is a curve used to computer private keys
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
