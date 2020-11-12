package iptbtester

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/pkg/constants"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

// GenesisInfo chains require information to start a single node with funds
type GenesisInfo struct {
	GenesisFile        string
	KeyFile            string
	WalletAddress      string
	MinerAddress       string
	SectorsDir         string
	PresealedSectorDir string
}

type idResult struct {
	ID string
}

// RequireGenesisFromSetup constructs the required information and files to build a single
// filecoin node from a genesis configuration file. The GenesisInfo can be used with MustImportGenesisMiner
func RequireGenesisFromSetup(t *testing.T, dir string, setupPath string) *GenesisInfo {
	cfg := ReadSetup(t, setupPath)
	return RequireGenesis(t, dir, cfg)
}

// RequireGenerateGenesis constructs the required information and files to build a single
// filecoin node with the provided funds. The GenesisInfo can be used with MustImportGenesisMiner
func RequireGenerateGenesis(t *testing.T, funds int64, dir string, genesisTime time.Time) *GenesisInfo {
	// Setup, generate a genesis and key file
	commCfgs, err := gengen.MakeCommitCfgs(1)
	require.NoError(t, err)
	cfg := &gengen.GenesisCfg{
		Seed:      0,
		KeysToGen: 1,
		PreallocatedFunds: []string{
			strconv.FormatInt(funds, 10),
		},
		Miners: []*gengen.CreateStorageMinerConfig{
			{
				Owner:            0,
				CommittedSectors: commCfgs,
				SealProofType:    constants.DevSealProofType,
				MarketBalance:    abi.NewTokenAmount(0),
			},
		},
		Network: "gfctest",
		Time:    uint64(genesisTime.Unix()),
	}

	return RequireGenesis(t, dir, cfg)
}

// ReadSetup reads genesis config from a setup file
func ReadSetup(t *testing.T, setupPath string) *gengen.GenesisCfg {
	configFile, err := os.Open(setupPath)
	if err != nil {
		t.Errorf("failed to open config file %s: %s", setupPath, err)
	}
	defer configFile.Close() // nolint: errcheck

	var cfg gengen.GenesisCfg
	if err := json.NewDecoder(configFile).Decode(&cfg); err != nil {
		t.Errorf("failed to parse config: %s", err)
	}
	return &cfg
}

// RequireGenesis generates a genesis block and metadata from config
func RequireGenesis(t *testing.T, dir string, cfg *gengen.GenesisCfg) *GenesisInfo {
	genfile, err := ioutil.TempFile(dir, "genesis.*.car")
	if err != nil {
		t.Fatal(err)
	}

	keyfile, err := ioutil.TempFile(dir, "wallet.*.key")
	if err != nil {
		t.Fatal(err)
	}

	info, err := gengen.GenGenesisCar(cfg, genfile)
	if err != nil {
		t.Fatal(err)
	}

	minerCfg := info.Miners[0]
	minerKeyIndex := minerCfg.Owner

	var wsr commands.WalletSerializeResult
	wsr.KeyInfo = append(wsr.KeyInfo, info.Keys[minerKeyIndex])
	if err := json.NewEncoder(keyfile).Encode(wsr); err != nil {
		t.Fatal(err)
	}

	walletAddr, err := info.Keys[minerKeyIndex].Address()
	if err != nil {
		t.Fatal(err)
	}

	minerAddr := minerCfg.Address

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

	tn.MustRunCmd(ctx, "venus", "config", "mining.minerAddress", fmt.Sprintf("\"%s\"", gi.MinerAddress))

	tn.MustRunCmd(ctx, "venus", "wallet", "import", gi.KeyFile)

	tn.MustRunCmd(ctx, "venus", "config", "wallet.defaultAddress", fmt.Sprintf("\"%s\"", gi.WalletAddress))

	// Get node id
	id := idResult{}
	tn.MustRunCmdJSON(ctx, &id, "venus", "id")

	// Update miner
	tn.MustRunCmd(ctx, "venus", "miner", "update-peerid", "--from="+gi.WalletAddress, "--gas-price=1", "--gas-limit=300", gi.MinerAddress, id.ID)
}

// MustInitWithGenesis init TestNode, passing in the `--genesisfile` flag, by calling MustInit
func (tn *TestNode) MustInitWithGenesis(ctx context.Context, genesisinfo *GenesisInfo, args ...string) *TestNode {
	genesisfileFlag := fmt.Sprintf("--genesisfile=%s", genesisinfo.GenesisFile)
	args = append(args, genesisfileFlag)

	if genesisinfo.KeyFile != "" {
		keyfileFlag := fmt.Sprintf("--wallet-keyfile=%s", genesisinfo.KeyFile)
		args = append(args, keyfileFlag)
	}

	if genesisinfo.MinerAddress != "" {
		minerActorAddressFlag := fmt.Sprintf("--miner-actor-address=%s", genesisinfo.MinerAddress)
		args = append(args, minerActorAddressFlag)
	}

	if genesisinfo.PresealedSectorDir != "" {
		presealedSectorDirFlag := fmt.Sprintf("--presealed-sectordir=%s", genesisinfo.PresealedSectorDir)
		args = append(args, presealedSectorDirFlag)
	}

	if genesisinfo.SectorsDir != "" {
		sectorDirFlag := fmt.Sprintf("--sectordir=%s", genesisinfo.SectorsDir)
		args = append(args, sectorDirFlag)
	}

	tn.MustInit(ctx, args...)
	return tn
}
