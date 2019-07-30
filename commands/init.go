package commands

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/ipfs/go-car"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
)

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(GenesisFile, "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
		cmdkit.StringOption(PeerKeyFile, "path of file containing key to use for new node's libp2p identity"),
		cmdkit.StringOption(WithMiner, "when set, creates a custom genesis block with a pre generated miner account, requires running the daemon using dev mode (--dev)"),
		cmdkit.StringOption(OptionSectorDir, "path of directory into which staged and sealed sectors will be written"),
		cmdkit.StringOption(DefaultAddress, "when set, sets the daemons's default address to the provided address"),
		cmdkit.UintOption(AutoSealIntervalSeconds, "when set to a number > 0, configures the daemon to check for and seal any staged sectors on an interval.").WithDefault(uint(120)),
		cmdkit.BoolOption(DevnetStaging, "when set, populates config bootstrap addrs with the dns multiaddrs of the staging devnet and other staging devnet specific bootstrap parameters."),
		cmdkit.BoolOption(DevnetNightly, "when set, populates config bootstrap addrs with the dns multiaddrs of the nightly devnet and other nightly devnet specific bootstrap parameters"),
		cmdkit.BoolOption(DevnetUser, "when set, populates config bootstrap addrs with the dns multiaddrs of the user devnet and other user devnet specific bootstrap parameters"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		newConfig, err := getConfigFromOptions(req.Options)
		if err != nil {
			return err
		}

		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err = paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}

		if err := re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)); err != nil {
			return err
		}
		if err := repo.InitFSRepo(repoDir, repo.Version, newConfig); err != nil {
			return err
		}
		rep, err := repo.OpenFSRepo(repoDir, repo.Version)
		if err != nil {
			return err
		}

		// The only error Close can return is that the repo has already been closed
		defer rep.Close() // nolint: errcheck

		genesisFileSource, _ := req.Options[GenesisFile].(string)
		genesisFile, err := loadGenesis(req.Context, rep, genesisFileSource)
		if err != nil {
			return err
		}

		autoSealIntervalSeconds, _ := req.Options[AutoSealIntervalSeconds].(uint)
		peerKeyFile, _ := req.Options[PeerKeyFile].(string)
		initopts, err := getNodeInitOpts(autoSealIntervalSeconds, peerKeyFile)
		if err != nil {
			return err
		}

		return node.Init(req.Context, rep, genesisFile, initopts...)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(initTextEncoder),
	},
}

func getConfigFromOptions(options cmdkit.OptMap) (*config.Config, error) {
	newConfig := config.NewDefaultConfig()

	if dir, ok := options[OptionSectorDir].(string); ok {
		newConfig.SectorBase.RootDir = dir
	}

	if m, ok := options[WithMiner].(string); ok {
		var err error
		newConfig.Mining.MinerAddress, err = address.NewFromString(m)
		if err != nil {
			return nil, err
		}
	}

	if m, ok := options[DefaultAddress].(string); ok {
		var err error
		newConfig.Wallet.DefaultAddress, err = address.NewFromString(m)
		if err != nil {
			return nil, err
		}
	}

	devnetTest, _ := options[DevnetStaging].(bool)
	devnetNightly, _ := options[DevnetNightly].(bool)
	devnetUser, _ := options[DevnetUser].(bool)
	if (devnetTest && devnetNightly) || (devnetTest && devnetUser) || (devnetNightly && devnetUser) {
		return nil, fmt.Errorf(`cannot specify more than one "devnet-" option`)
	}

	// Setup devnet specific config options.
	if devnetTest || devnetNightly || devnetUser {
		newConfig.Bootstrap.MinPeerThreshold = 1
		newConfig.Bootstrap.Period = "10s"
	}

	// Setup devnet staging specific config options.
	if devnetTest {
		newConfig.Bootstrap.Addresses = fixtures.DevnetStagingBootstrapAddrs
		newConfig.Net = "devnet-staging"
	}

	// Setup devnet nightly specific config options.
	if devnetNightly {
		newConfig.Bootstrap.Addresses = fixtures.DevnetNightlyBootstrapAddrs
		newConfig.Net = "devnet-nightly"
	}

	// Setup devnet user specific config options.
	if devnetUser {
		newConfig.Bootstrap.Addresses = fixtures.DevnetUserBootstrapAddrs
		newConfig.Net = "devnet-user"
	}

	return newConfig, nil
}

func initTextEncoder(req *cmds.Request, w io.Writer, val interface{}) error {
	_, err := fmt.Fprintf(w, val.(string))
	return err
}

func loadGenesis(ctx context.Context, rep repo.Repo, sourceName string) (consensus.GenesisInitFunc, error) {
	if sourceName == "" {
		return consensus.MakeGenesisFunc(consensus.ProofsMode(types.LiveProofsMode)), nil
	}

	sourceURL, err := url.Parse(sourceName)
	if err != nil {
		return nil, fmt.Errorf("invalid filepath or URL for genesis file: %s", sourceURL)
	}

	var source io.ReadCloser
	if sourceURL.Scheme == "http" || sourceURL.Scheme == "https" {
		// NOTE: This code is temporary. It allows downloading a genesis block via HTTP(S) to be able to join a
		// recently deployed staging devnet.
		response, err := http.Get(sourceName)
		if err != nil {
			return nil, err
		}
		source = response.Body
	} else if sourceURL.Scheme != "" {
		return nil, fmt.Errorf("unsupported protocol for genesis file: %s", sourceURL.Scheme)
	} else {
		file, err := os.Open(sourceName)
		if err != nil {
			return nil, err
		}
		source = file
	}
	defer source.Close() // nolint: errcheck

	bs := blockstore.NewBlockstore(rep.Datastore())
	ch, err := car.LoadCar(bs, source)
	if err != nil {
		return nil, err
	}

	if len(ch.Roots) != 1 {
		return nil, fmt.Errorf("expected car with only a single root")
	}

	gif := func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
		var blk types.Block

		if err := cst.Get(ctx, ch.Roots[0], &blk); err != nil {
			return nil, err
		}

		return &blk, nil
	}

	return gif, nil
}

func getNodeInitOpts(autoSealIntervalSeconds uint, peerKeyFile string) ([]node.InitOpt, error) {
	var initOpts []node.InitOpt
	if peerKeyFile != "" {
		data, err := ioutil.ReadFile(peerKeyFile)
		if err != nil {
			return nil, err
		}
		peerKey, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return nil, err
		}
		initOpts = append(initOpts, node.PeerKeyOpt(peerKey))
	}

	initOpts = append(initOpts, node.AutoSealIntervalSecondsOpt(autoSealIntervalSeconds))

	return initOpts, nil
}
