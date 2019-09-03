package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
	"github.com/filecoin-project/go-filecoin/types"
)

type MinerConfig struct {
	CommonConfig
	FaucetURL        string
	AutoSealInterval int
	Collateral       int
	AskPrice         string
	AskExpiry        int
	SectorSize       types.BytesAmount
}

type MinerProfile struct {
	config MinerConfig
	runner FASTRunner
}

func NewMinerProfile(configfile string) (Profile, error) {
	cf, err := os.Open(configfile)
	if err != nil {
		return nil, errors.Wrapf(err, "config file %s", configfile)
	}

	defer cf.Close()

	dec := json.NewDecoder(cf)

	var config MinerConfig
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

	return &MinerProfile{config, runner}, nil
}

func (p *MinerProfile) Pre() error {
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

	cfg.Observability.Metrics.PrometheusEnabled = true

	// IPTB changes this to loopback and a random port
	cfg.Swarm.Address = "/ip4/0.0.0.0/tcp/6000"

	if err := node.WriteConfig(cfg); err != nil {
		return err
	}

	return nil
}

func (p *MinerProfile) Daemon() error {
	args := []string{}
	for _, argfn := range p.runner.ProcessArgs.DaemonOpts {
		args = append(args, argfn()...)
	}

	fmt.Println(strings.Join(args, " "))

	return nil
}

func (p *MinerProfile) Post() error {
	ctx := context.Background()
	miner, err := GetNode(ctx, lpfc.PluginName, p.runner.WorkingDir, p.runner.PluginOptions, p.runner.ProcessArgs)
	if err != nil {
		return err
	}
	defer miner.DumpLastOutput(os.Stdout)

	miningStatus, err := miner.MiningStatus(ctx)
	if err != nil {
		return err
	}
	if mining := miningStatus.Active; !mining {
		if err := FaucetRequest(ctx, miner, p.config.FaucetURL); err != nil {
			return err
		}

		collateral := big.NewInt(int64(p.config.Collateral))
		price, _, err := big.ParseFloat(p.config.AskPrice, 10, 128, big.AwayFromZero)
		if err != nil {
			return err
		}

		expiry := big.NewInt(int64(p.config.AskExpiry))

		_, err = series.CreateStorageMinerWithAsk(ctx, miner, collateral, price, expiry, &p.config.SectorSize)
		if err != nil {
			return err
		}

		if err := miner.MiningStart(ctx); err != nil {
			return err
		}
	}
	return nil
}

func FaucetRequest(ctx context.Context, p *fast.Filecoin, uri string) error {
	var toAddr address.Address
	if err := p.ConfigGet(ctx, "wallet.defaultAddress", &toAddr); err != nil {
		return err
	}

	data := url.Values{}
	data.Set("target", toAddr.String())

	resp, err := http.PostForm(uri, data)
	if err != nil {
		return err
	}

	msgcid := resp.Header.Get("Message-Cid")
	mcid, err := cid.Decode(msgcid)
	if err != nil {
		return fmt.Errorf("Failed to decode %s: %s", msgcid, mcid)
	}

	if _, err := p.MessageWait(ctx, mcid); err != nil {
		return err
	}

	return nil
}

func (p *MinerProfile) Main() error { return nil }
