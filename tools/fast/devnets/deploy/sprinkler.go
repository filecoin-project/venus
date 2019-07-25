package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"io/ioutil"
	//"net/http"
	//"net/url"
	"os"
	"time"

	files "github.com/ipfs/go-ipfs-files"
	//"github.com/multiformats/go-multiaddr"
	//"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	//"github.com/filecoin-project/go-filecoin/address"
	//"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/porcelain"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
	"github.com/filecoin-project/go-filecoin/types"
)

type SprinklerConfig struct {
	CommonConfig
	FaucetURL string
	MaxPrice  string
}

type SprinklerPro struct {
	config SprinklerConfig
	runner FASTRunner
}

func NewSprinklerProfile(configfile string) (Profile, error) {
	cf, err := os.Open(configfile)
	if err != nil {
		return nil, errors.Wrapf(err, "config file %s", configfile)
	}

	defer cf.Close()

	dec := json.NewDecoder(cf)

	var config SprinklerConfig
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

	return &SprinklerPro{config, runner}, nil
}

func (p *SprinklerPro) Pre() error { return nil }
func (p *SprinklerPro) Daemon() error { return nil }
func (p *SprinklerPro) Post() error { return nil }
func (p *SprinklerPro) Main() error {
	ctx := context.Background()
	tmpdir, err := ioutil.TempDir("", "deal-maker")
	if err != nil {
		return err
	}

	processArgs := fast.FilecoinOpts{
		InitOpts:   p.runner.ProcessArgs.InitOpts,
		DaemonOpts: p.runner.ProcessArgs.DaemonOpts,
	}

	node, err := GetNode(ctx, lpfc.PluginName, tmpdir, p.runner.PluginOptions, processArgs)
	if err != nil {
		return err
	}

	if o, err := node.InitDaemon(ctx); err != nil {
		io.Copy(os.Stdout, o.Stdout())
		io.Copy(os.Stdout, o.Stderr())
		return err
	}

	cfg, err := node.Config()
	if err != nil {
		return err
	}

	cfg.Observability.Metrics.PrometheusEnabled = true

	// IPTB changes this to loopback
	cfg.Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	if err := node.WriteConfig(cfg); err != nil {
		return err
	}

	if o, err := node.StartDaemon(ctx, true); err != nil {
		io.Copy(os.Stdout, o.Stdout())
		io.Copy(os.Stdout, o.Stderr())
		return err
	}
	defer node.StopDaemon(ctx)

	if err := FaucetRequest(ctx, node, p.config.FaucetURL); err != nil {
		return err
	}

	maxPrice, _ := types.NewAttoFILFromFILString(p.config.MaxPrice)

	dec, err := node.ClientListAsks(ctx)
	if err != nil {
		return err
	}

	var asks []porcelain.Ask
	for {
		var ask porcelain.Ask

		err := dec.Decode(&ask)
		if err != nil && err != io.EOF {
			continue
		}

		if err == io.EOF {
			break
		}

		if !ask.Price.GreaterThan(maxPrice) {
			asks = append(asks, ask)
		}
	}

	for _, ask := range asks {
		var data bytes.Buffer
		dataReader := io.LimitReader(rand.Reader, int64(256))
		dataReader = io.TeeReader(dataReader, &data)
		_, deal, err := series.ImportAndStoreWithDuration(ctx, node, ask, 256, files.NewReaderFile(dataReader))
		if err != nil {
			return err
		}

		err = series.WaitForDealState(ctx, node, deal, storagedeal.Complete)
		if err != nil {
			return err
		}
	}
	return nil
}

