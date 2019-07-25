package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs-files"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

type GenesisConfig struct {
	CommonConfig
	WalletFile   string
	MinerAddress address.Address
}

type GenesisProfile struct {
	config GenesisConfig
	runner FASTRunner
}

func NewGenesisProfile(configfile string) (Profile, error) {
	cf, err := os.Open(configfile)
	if err != nil {
		return nil, errors.Wrapf(err, "config file %s", configfile)
	}

	defer cf.Close()

	dec := json.NewDecoder(cf)

	var config GenesisConfig
	if err := dec.Decode(&config); err != nil {
		return nil, errors.Wrap(err, "config")
	}

	blocktime, err := time.ParseDuration(config.BlockTime)
	if err != nil {
		return nil, err
	}

	runner := FASTRunner{
		WorkingDir: config.WorkingDir,
		ProcessArgs: fast.FilecoinOpts{
			InitOpts: []fast.ProcessInitOption{
				fast.POGenesisFile(config.GenesisCarFile),
				NetworkPO(config.Network),
				fast.POPeerKeyFile(config.PeerkeyFile),
			},
			DaemonOpts: []fast.ProcessDaemonOption{
				fast.POBlockTime(blocktime),
			},
		},
		PluginOptions: map[string]string{
			lpfc.AttrLogJSON:  config.LogJSON,
			lpfc.AttrLogLevel: config.LogLevel,
		},
	}

	// TODO(tperson) stat the wallet file?

	return &GenesisProfile{config, runner}, nil
}

func (p *GenesisProfile) Pre() error {
	ctx := context.Background()

	node, err := GetNode(ctx, lpfc.PluginName, p.runner.WorkingDir, p.runner.PluginOptions, p.runner.ProcessArgs)
	if err != nil {
		return err
	}

	if _, err := os.Stat(p.runner.WorkingDir + "/repo"); os.IsNotExist(err) {
		if o, err := node.InitDaemon(ctx); err != nil {
			io.Copy(os.Stdout, o.Stdout())
			io.Copy(os.Stdout, o.Stderr())
			return err
		}
	} else if err != nil {
		return err
	}

	cfg, err := node.Config()
	if err != nil {
		return err
	}

	// API needs to be accessible for the faucet
	cfg.API.Address = "/ip4/0.0.0.0/tcp/3453"

	// IPTB changes this to loopback and a random port
	cfg.Swarm.Address = "/ip4/0.0.0.0/tcp/6000"
	cfg.Observability.Metrics.PrometheusEnabled = true

	if err := node.WriteConfig(cfg); err != nil {
		return err
	}

	return nil
}

func (p *GenesisProfile) Daemon() error {
	args := []string{}
	for _, argfn := range p.runner.ProcessArgs.DaemonOpts {
		args = append(args, argfn()...)
	}

	fmt.Println(strings.Join(args, " "))

	return nil
}

func (p *GenesisProfile) Post() error {
	ctx := context.Background()
	node, err := GetNode(ctx, lpfc.PluginName, p.runner.WorkingDir, p.runner.PluginOptions, p.runner.ProcessArgs)
	if err != nil {
		return err
	}

	defer node.DumpLastOutput(os.Stdout)

	miningStatus, err := node.MiningStatus(ctx)
	if err != nil {
		return err
	}
	if mining := miningStatus.Active; !mining {
		walletReader, err := os.Open(p.config.WalletFile)
		if err != nil {
			return errors.Wrapf(err, "wallet file %s", p.config.WalletFile)
		}

		defer walletReader.Close()

		wallet, err := node.WalletImport(ctx, files.NewReaderFile(walletReader))
		if err != nil {
			return err
		}

		if err := node.ConfigSet(ctx, "wallet.defaultAddress", wallet[0].String()); err != nil {
			return err
		}

		// Miner address must be set after the node is started otherwise the node fails to start due to not being
		// able to find the actor?
		// Error: failed to initialize sector builder:
		// failed to get sector size for miner w/address t2ifoswatrfyalykq3o2b5r3q2t5kppur3z7q3ftq:
		// query and deserialize failed: 'getSectorSize' query message failed:
		// querymethod returned an error: failed to get To actor: actor not found
		if err := node.ConfigSet(ctx, "mining.minerAddress", p.config.MinerAddress.String()); err != nil {
			return err
		}

		details, err := node.ID(ctx)
		if err != nil {
			return err
		}

		// Miner Update
		_, err = node.MinerUpdatePeerid(ctx, p.config.MinerAddress, details.ID, fast.AOFromAddr(wallet[0]), fast.AOPrice(big.NewFloat(0.001)), fast.AOLimit(300))
		if err != nil {
			return err
		}

		if err := node.MiningStart(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (p *GenesisProfile) Main() error { return nil }
