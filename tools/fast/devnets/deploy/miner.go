package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs-files"
	//"github.com/multiformats/go-multiaddr"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/protocol/storage/storagedeal"
	"github.com/filecoin-project/go-filecoin/tools/fast"
	"github.com/filecoin-project/go-filecoin/tools/fast/series"
	lpfc "github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin/local"
)

type MinerConfig struct {
	CommonConfig
	FaucetURL        string
	AutoSealInterval int
	Collateral       int
	AskPrice         string
	AskExpiry        int
	SectorSize       int
}

type MinerProfile struct {
	config MinerConfig
	foobar Foobar
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

	foobar := Foobar{
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

	return &MinerProfile{config, foobar}, nil
}

func (p *MinerProfile) Pre() error {
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

func (p *MinerProfile) Daemon() error {
	args := []string{}
	for _, argfn := range p.foobar.ProcessArgs.DaemonOpts {
		args = append(args, argfn()...)
	}

	fmt.Println(strings.Join(args, " "))

	return nil
}

func (p *MinerProfile) Post() error {
	ctx := context.Background()
	miner, err := GetNode(ctx, lpfc.PluginName, p.foobar.WorkingDir, p.foobar.PluginOptions, p.foobar.ProcessArgs)
	if err != nil {
		return err
	}

	client, err := GetClientNode(ctx, p.foobar)
	if err != nil {
		return err
	}

	defer client.StopDaemon(ctx)

	if err := FaucetRequest(ctx, miner, p.config.FaucetURL); err != nil {
		return err
	}

	if err := FaucetRequest(ctx, client, p.config.FaucetURL); err != nil {
		return err
	}

	collateral := big.NewInt(int64(p.config.Collateral))
	price, _, err := big.ParseFloat(p.config.AskPrice, 10, 128, big.AwayFromZero)
	if err != nil {
		return err
	}

	expiry := big.NewInt(int64(p.config.AskExpiry))

	ask, err := series.CreateStorageMinerWithAsk(ctx, miner, collateral, price, expiry)
	if err != nil {
		return err
	}

	if err := miner.MiningStart(ctx); err != nil {
		return err
	}

	if err := series.Connect(ctx, client, miner); err != nil {
		return err
	}

	var data bytes.Buffer
	dataReader := io.LimitReader(rand.Reader, int64(p.config.SectorSize))
	dataReader = io.TeeReader(dataReader, &data)
	_, deal, err := series.ImportAndStore(ctx, client, ask, files.NewReaderFile(dataReader))
	if err != nil {
		return err
	}

	if err := series.WaitForDealState(ctx, client, deal, storagedeal.Complete); err != nil {
		return err
	}

	return nil
}

// GetClientNode return a new node that is used to make deals with the miner so it can have power
func GetClientNode(ctx context.Context, foobar Foobar) (*fast.Filecoin, error) {
	tmpdir, err := ioutil.TempDir("", "deal-maker")
	if err != nil {
		return nil, err
	}

	processArgs := fast.FilecoinOpts{
		InitOpts:   foobar.ProcessArgs.InitOpts,
		DaemonOpts: foobar.ProcessArgs.DaemonOpts,
	}

	client, err := GetNode(ctx, lpfc.PluginName, tmpdir, foobar.PluginOptions, processArgs)
	if err != nil {
		return nil, err
	}

	if o, err := client.InitDaemon(ctx); err != nil {
		io.Copy(os.Stdout, o.Stdout())
		io.Copy(os.Stdout, o.Stderr())
		return nil, err
	}

	cfg, err := client.Config()
	if err != nil {
		return nil, err
	}

	cfg.Observability.Metrics.PrometheusEnabled = true

	// IPTB changes this to loopback
	cfg.Swarm.Address = "/ip4/0.0.0.0/tcp/0"

	if err := client.WriteConfig(cfg); err != nil {
		return nil, err
	}

	if o, err := client.StartDaemon(ctx, true); err != nil {
		io.Copy(os.Stdout, o.Stdout())
		io.Copy(os.Stdout, o.Stderr())
		return nil, err
	}

	return client, nil
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
