package iptbtester

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/filecoin-project/go-filecoin/commands"
	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
)

// GenesisInfo chains require information to start a single node with funds
type GenesisInfo struct {
	GenesisFile   string
	KeyFile       string
	WalletAddress string
	MinerAddress  string
}

type idResult struct {
	ID string
}

// MustGenerateGenesis constructs the requires information and files to build a single
// filecoin node with the provided funds. The GenesisInfo can be used with MustImportGenesisMiner
func MustGenerateGenesis(t *testing.T, funds int64, dir string) *GenesisInfo {
	// Setup, generate a genesis and key file
	cfg := &gengen.GenesisCfg{
		Keys: 1,
		PreAlloc: []string{
			strconv.FormatInt(funds, 10),
		},
		Miners: []gengen.Miner{
			{
				Owner:               0,
				NumCommittedSectors: 1,
			},
		},
	}

	genfile, err := ioutil.TempFile(dir, "genesis.*.car")
	if err != nil {
		t.Fatal(err)
	}

	keyfile, err := ioutil.TempFile(dir, "wallet.*.key")
	if err != nil {
		t.Fatal(err)
	}

	info, err := gengen.GenGenesisCar(cfg, genfile, 0)
	if err != nil {
		t.Fatal(err)
	}

	var wsr commands.WalletSerializeResult
	wsr.KeyInfo = append(wsr.KeyInfo, info.Keys[0])
	if err := json.NewEncoder(keyfile).Encode(wsr); err != nil {
		t.Fatal(err)
	}

	walletAddr, err := info.Keys[0].Address()
	if err != nil {
		t.Fatal(err)
	}

	minerAddr := info.Miners[0].Address

	return &GenesisInfo{
		GenesisFile:   genfile.Name(),
		KeyFile:       keyfile.Name(),
		WalletAddress: walletAddr.String(),
		MinerAddress:  minerAddr.String(),
	}
}

// MustImportGenesisMiner configures a node from the GenesisInfo and starts it mining.
// The node should already be initialized with the GenesisFile, and be should started.
func MustImportGenesisMiner(tn *TestNode, gi *GenesisInfo) {
	ctx := context.Background()

	tn.MustRunCmd(ctx, "go-filecoin", "config", "mining.minerAddress", fmt.Sprintf("\"%s\"", gi.MinerAddress))

	tn.MustRunCmd(ctx, "go-filecoin", "wallet", "import", gi.KeyFile)

	tn.MustRunCmd(ctx, "go-filecoin", "config", "wallet.defaultAddress", fmt.Sprintf("\"%s\"", gi.WalletAddress))

	// Get node id
	id := idResult{}
	tn.MustRunCmdJSON(ctx, &id, "go-filecoin", "id")

	// Update miner
	tn.MustRunCmd(ctx, "go-filecoin", "miner", "update-peerid", "--from="+gi.WalletAddress, "--gas-price=1", "--gas-limit=300", gi.MinerAddress, id.ID)
}

// MustInitWithGenesis init TestNode, passing in the `--genesisfile` flag, by calling MustInit
func (tn *TestNode) MustInitWithGenesis(ctx context.Context, genesisinfo *GenesisInfo, args ...string) *TestNode {
	genesisfileFlag := fmt.Sprintf("--genesisfile=%s", genesisinfo.GenesisFile)
	args = append(args, genesisfileFlag)
	tn.MustInit(ctx, args...)
	return tn
}
