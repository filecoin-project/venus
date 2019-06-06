package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/tools/fast"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

type BootstrapConfig struct {
	CommonConfig
	SwarmListen      string
	SwarmRelayPublic string
}

type BootstrapProfile struct {
	config BootstrapConfig
	foobar Foobar
}

func NewBootstrapProfile(configfile string) (Profile, error) {
	cf, err := os.Open(configfile)
	if err != nil {
		return nil, errors.Wrapf(err, "config file %s", configfile)
	}

	defer cf.Close()

	dec := json.NewDecoder(cf)

	var config BootstrapConfig
	if err := dec.Decode(&config); err != nil {
		return nil, errors.Wrap(err, "config")
	}

	blocktime, err := time.ParseDuration(config.BlockTime)
	if err != nil {
		return nil, err
	}

	swarmlisten, err := multiaddr.NewMultiaddr(config.SwarmListen)
	if err != nil {
		return nil, err
	}

	swarmrelaypublic, err := multiaddr.NewMultiaddr(config.SwarmRelayPublic)
	if err != nil {
		return nil, err
	}

	foobar := Foobar{
		WorkingDir: config.WorkingDir,
		ProcessArgs: fast.FilecoinOpts{
			InitOpts: []fast.ProcessInitOption{
				fast.POGenesisFile(config.GenesisCarFile),
				NetworkPO(config.Network),
				fast.POPeerKeyFile(config.PeerkeyFile),
			},
			DaemonOpts: []fast.ProcessDaemonOption{
				fast.POBlockTime(blocktime),
				fast.POIsRelay(),
				fast.POSwarmListen(swarmlisten),
				fast.POSwarmRelayPublic(swarmrelaypublic),
			},
		},
		PluginOptions: map[string]string{
			lpfc.AttrLogJSON:  config.LogJSON,
			lpfc.AttrLogLevel: config.LogLevel,
		},
	}

	return &BootstrapProfile{config, foobar}, nil
}

func (p *BootstrapProfile) Pre() error {
	ctx := context.Background()

	node, err := GetNode(ctx, lpfc.PluginName, p.foobar.WorkingDir, p.foobar.PluginOptions, p.foobar.ProcessArgs)
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

	// IPTB changes this to loopback and a random port
	cfg.Swarm.Address = "/ip4/0.0.0.0/tcp/6000"

	if err := node.WriteConfig(cfg); err != nil {
		return err
	}

	return nil
}

func (p *BootstrapProfile) Daemon() error {
	args := []string{}
	for _, argfn := range p.foobar.ProcessArgs.DaemonOpts {
		args = append(args, argfn()...)
	}

	fmt.Println(strings.Join(args, " "))

	return nil
}

func (p *BootstrapProfile) Post() error {
	ctx := context.Background()
	node, err := GetNode(ctx, lpfc.PluginName, p.foobar.WorkingDir, p.foobar.PluginOptions, p.foobar.ProcessArgs)
	if err != nil {
		return err
	}

	defer node.DumpLastOutput(os.Stdout)

	return nil
}
