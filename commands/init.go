package commands

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	hamt "gx/ipfs/QmNf3wujpV2Y7Lnj2hy2UrmuX8bhMDStRHbnSLh7Ypf36h/go-hamt-ipld"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRu7tiRnFk9mMPpVECQTBQJqXtmG132jJxA1w9A7TtpBz/go-ipfs-blockstore"
	"gx/ipfs/QmTW4SdgBWq9GjsBsHeUx8WuGxzhgzAf88UMH2w62PC8yK/go-libp2p-crypto"
	"gx/ipfs/QmUGpiTCKct5s1F7jaAnY9KJmoo7Qm1R2uhSjq5iHDSUMn/go-car"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cmds "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

// DaemonInitConfig is a helper struct to configure the init process of a daemon.
type DaemonInitConfig struct {
	// GenesisFile, path to a file containing archive of genesis block DAG data
	GenesisFile string
	// RepoDir, path to the repo of the node on disk.
	RepoDir string
	// PeerKeyFile is the path to a file containing a libp2p peer id key
	PeerKeyFile string
	// WithMiner, if set, sets the config value for the local miner to this address.
	WithMiner address.Address
	// DevnetTest, if set, sets the config to enable bootstrapping to the test devnet.
	DevnetTest bool
	// DevnetNightly, if set, sets the config to enable bootstrapping to the nightly devnet.
	DevnetNightly bool
	// DevnetUser, if set, sets the config to enable bootstrapping to the user devnet.
	DevnetUser bool
	// AutoSealIntervalSeconds, when set, configures the daemon to check for and seal any staged sectors on an interval
	AutoSealIntervalSeconds uint
	DefaultAddress          address.Address
}

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(GenesisFile, "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
		cmdkit.StringOption(PeerKeyFile, "path of file containing key to use for new node's libp2p identity"),
		cmdkit.StringOption(WithMiner, "when set, creates a custom genesis block with a pre generated miner account, requires running the daemon using dev mode (--dev)"),
		cmdkit.StringOption(DefaultAddress, "when set, sets the daemons's default address to the provided address"),
		cmdkit.UintOption(AutoSealIntervalSeconds, "when set to a number > 0, configures the daemon to check for and seal any staged sectors on an interval.").WithDefault(uint(120)),
		cmdkit.BoolOption(DevnetTest, "when set, populates config bootstrap addrs with the dns multiaddrs of the test devnet and other test devnet specific bootstrap parameters."),
		cmdkit.BoolOption(DevnetNightly, "when set, populates config bootstrap addrs with the dns multiaddrs of the nightly devnet and other nightly devnet specific bootstrap parameters"),
		cmdkit.BoolOption(DevnetUser, "when set, populates config bootstrap addrs with the dns multiaddrs of the user devnet and other user devnet specific bootstrap parameters"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		repoDir := getRepoDir(req)
		if err := re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)); err != nil {
			return err
		}

		genesisFile, _ := req.Options[GenesisFile].(string)
		peerKeyFile, _ := req.Options[PeerKeyFile].(string)
		autoSealIntervalSeconds, _ := req.Options[AutoSealIntervalSeconds].(uint)
		devnetTest, _ := req.Options[DevnetTest].(bool)
		devnetNightly, _ := req.Options[DevnetNightly].(bool)
		devnetUser, _ := req.Options[DevnetUser].(bool)

		var withMiner address.Address
		if m, ok := req.Options[WithMiner].(string); ok {
			var err error
			withMiner, err = address.NewFromString(m)
			if err != nil {
				return err
			}
		}

		var defaultAddress address.Address
		if m, ok := req.Options[DefaultAddress].(string); ok {
			var err error
			defaultAddress, err = address.NewFromString(m)
			if err != nil {
				return err
			}
		}

		// load configuration options
		cfg := &DaemonInitConfig{
			AutoSealIntervalSeconds: autoSealIntervalSeconds,
			DefaultAddress:          defaultAddress,
			DevnetNightly:           devnetNightly,
			DevnetTest:              devnetTest,
			DevnetUser:              devnetUser,
			GenesisFile:             genesisFile,
			PeerKeyFile:             peerKeyFile,
			RepoDir:                 repoDir,
			WithMiner:               withMiner,
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
			genCid, err := loadGenesis(rep, cfg.GenesisFile)
			if err != nil {
				return err
			}

			gif = func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
				var blk types.Block

				if err := cst.Get(req.Context, genCid, &blk); err != nil {
					return nil, err
				}

				return &blk, nil
			}
		}

		// TODO: don't create the repo if this fails
		return node.Init(req.Context, rep, gif, initopts...)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(initTextEncoder),
	},
}

func initTextEncoder(req *cmds.Request, w io.Writer, val interface{}) error {
	_, err := fmt.Fprintf(w, val.(string))
	return err
}

func getRepoDir(req *cmds.Request) string {
	envdir := os.Getenv("FIL_PATH")

	repodir, ok := req.Options[OptionRepoDir].(string)
	if ok {
		return repodir
	}

	if envdir != "" {
		return envdir
	}

	return "~/.filecoin"
}

func loadPeerKey(fname string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(data)
}

func loadGenesis(rep repo.Repo, sourceName string) (cid.Cid, error) {
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
