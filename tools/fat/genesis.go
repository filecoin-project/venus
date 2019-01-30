package fat

import (
	"encoding/json"
	"io/ioutil"
	"strconv"

	"github.com/filecoin-project/go-filecoin/address"
	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
)

// GenesisInfo chains require information to start a single node with funds
type GenesisInfo struct {
	Path          string
	KeyFile       string
	WalletAddress address.Address
	MinerAddress  address.Address
}

// GenerateGenesis creates genesis infor with fund `funds` in directory `dir`.
func GenerateGenesis(funds int64, dir string) (*GenesisInfo, error) {
	// Setup, generate a genesis and key file
	cfg := &gengen.GenesisCfg{
		Keys: 1,
		PreAlloc: []string{
			strconv.FormatInt(funds, 10),
		},
		Miners: []gengen.Miner{
			{
				Owner: 0,
				Power: 1,
			},
		},
	}

	genfile, err := ioutil.TempFile(dir, "genesis.*.car")
	if err != nil {
		return nil, err
	}

	keyfile, err := ioutil.TempFile(dir, "wallet.*.key")
	if err != nil {
		return nil, err
	}

	info, err := gengen.GenGenesisCar(cfg, genfile, 0)
	if err != nil {
		return nil, err
	}

	key := info.Keys[0]
	if err := json.NewEncoder(keyfile).Encode(key); err != nil {
		return nil, err
	}

	walletAddr, err := key.Address()
	if err != nil {
		return nil, err
	}

	minerAddr := info.Miners[0].Address

	return &GenesisInfo{
		Path:          genfile.Name(),
		KeyFile:       keyfile.Name(),
		WalletAddress: walletAddr,
		MinerAddress:  minerAddr,
	}, nil
}
