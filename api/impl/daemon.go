package impl

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	hamt "gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	blockstore "gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	crypto "gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	car "gx/ipfs/QmUGpiTCKct5s1F7jaAnY9KJmoo7Qm1R2uhSjq5iHDSUMn/go-car"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

const (
	// SECP256K1 is a curve used to compute private keys
	SECP256K1 = "secp256k1"
)

type nodeDaemon struct {
	api *nodeAPI
}

func newNodeDaemon(api *nodeAPI) *nodeDaemon {
	return &nodeDaemon{api: api}
}

// Init, initializes everything needed to run a daemon, including the disk storage.
func (nd *nodeDaemon) Init(ctx context.Context, opts ...api.DaemonInitOpt) error {
	// load configuration options
	cfg := &api.DaemonInitConfig{}
	for _, o := range opts {
		o(cfg)
	}

	if err := repo.InitFSRepo(cfg.RepoDir, config.NewDefaultConfig()); err != nil {
		return err
	}

	rep, err := repo.OpenFSRepo(cfg.RepoDir)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := rep.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			} else {
				err = errors.Wrap(err, closeErr.Error())
			}
		} // else err may be set and returned as normal
	}()

	gif := consensus.DefaultGenesis

	var initopts []node.InitOpt
	if cfg.PeerKeyFile != "" {
		peerKey, err := loadPeerKey(cfg.PeerKeyFile)
		if err != nil {
			return err
		}
		initopts = append(initopts, node.PeerKeyOpt(peerKey))
	}

	initopts = append(initopts, node.AutoSealIntervalSecondsOpt(cfg.AutoSealIntervalSeconds))

	if (cfg.DevnetTest && cfg.DevnetNightly) || (cfg.DevnetTest && cfg.DevnetUser) || (cfg.DevnetNightly && cfg.DevnetUser) {
		return fmt.Errorf(`cannot specify more than one "devnet-" option`)
	}

	newConfig := rep.Config()

	newConfig.Mining.MinerAddress = cfg.WithMiner

	newConfig.Wallet.DefaultAddress = cfg.DefaultAddress

	// Setup devnet test specific config options.
	if cfg.DevnetTest {
		newConfig.Bootstrap.Addresses = fixtures.DevnetTestBootstrapAddrs
		newConfig.Bootstrap.MinPeerThreshold = 1
		newConfig.Bootstrap.Period = "10s"
		newConfig.Net = "devnet-test"
	}

	// Setup devnet nightly specific config options.
	if cfg.DevnetNightly {
		newConfig.Bootstrap.Addresses = fixtures.DevnetNightlyBootstrapAddrs
		newConfig.Bootstrap.MinPeerThreshold = 1
		newConfig.Bootstrap.Period = "10s"
		newConfig.Net = "devnet-nightly"
	}

	// Setup devnet user specific config options.
	if cfg.DevnetUser {
		newConfig.Bootstrap.Addresses = fixtures.DevnetUserBootstrapAddrs
		newConfig.Bootstrap.MinPeerThreshold = 1
		newConfig.Bootstrap.Period = "10s"
		newConfig.Net = "devnet-user"
	}

	if err := rep.ReplaceConfig(newConfig); err != nil {
		return err
	}

	switch {
	case cfg.GenesisFile != "":
		// TODO: this feels a little wonky, I think the InitGenesis interface might need some tweaking
		genCid, err := LoadGenesis(rep, cfg.GenesisFile)
		if err != nil {
			return err
		}

		gif = func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
			var blk types.Block

			if err := cst.Get(ctx, genCid, &blk); err != nil {
				return nil, err
			}

			return &blk, nil
		}
	}

	// TODO: don't create the repo if this fails
	return node.Init(ctx, rep, gif, initopts...)
}

func loadPeerKey(fname string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(data)
}

// LoadGenesis gets the genesis block from either a local car file or an HTTP(S) URL.
func LoadGenesis(rep repo.Repo, sourceName string) (cid.Cid, error) {
	var source io.ReadCloser

	sourceURL, err := url.Parse(sourceName)
	if err != nil {
		return cid.Undef, fmt.Errorf("invalid filepath or URL for genesis file: %s", sourceURL)
	}
	if sourceURL.Scheme == "http" || sourceURL.Scheme == "https" {
		// NOTE: This code is temporary. It allows downloading a genesis block via HTTP(S) to be able to join a
		// recently deployed test devnet.
		response, err := http.Get(sourceName)
		if err != nil {
			return cid.Undef, err
		}
		source = response.Body
	} else if sourceURL.Scheme != "" {
		return cid.Undef, fmt.Errorf("unsupported protocol for genesis file: %s", sourceURL.Scheme)
	} else {
		file, err := os.Open(sourceName)
		if err != nil {
			return cid.Undef, err
		}
		source = file
	}

	defer source.Close() // nolint: errcheck

	bs := blockstore.NewBlockstore(rep.Datastore())

	ch, err := car.LoadCar(bs, source)
	if err != nil {
		return cid.Undef, err
	}

	if len(ch.Roots) != 1 {
		return cid.Undef, fmt.Errorf("expected car with only a single root")
	}

	return ch.Roots[0], nil
}
