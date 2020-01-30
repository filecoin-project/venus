package commands

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-car"
	"github.com/ipfs/go-hamt-ipld"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

var logInit = logging.Logger("commands/init")

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(GenesisFile, "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
		cmdkit.StringOption(PeerKeyFile, "path of file containing key to use for new node's libp2p identity"),
		cmdkit.StringOption(WithMiner, "when set, creates a custom genesis block  a pre generated miner account, requires running the daemon using dev mode (--dev)"),
		cmdkit.StringOption(OptionSectorDir, "path of directory into which staged and sealed sectors will be written"),
		cmdkit.StringOption(DefaultAddress, "when set, sets the daemons's default address to the provided address"),
		cmdkit.UintOption(AutoSealIntervalSeconds, "when set to a number > 0, configures the daemon to check for and seal any staged sectors on an interval.").WithDefault(uint(120)),
		cmdkit.BoolOption(DevnetStaging, "when set, populates config bootstrap addrs with the dns multiaddrs of the staging devnet and other staging devnet specific bootstrap parameters."),
		cmdkit.BoolOption(DevnetNightly, "when set, populates config bootstrap addrs with the dns multiaddrs of the nightly devnet and other nightly devnet specific bootstrap parameters"),
		cmdkit.BoolOption(DevnetUser, "when set, populates config bootstrap addrs with the dns multiaddrs of the user devnet and other user devnet specific bootstrap parameters"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err := paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}

		if err := re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)); err != nil {
			return err
		}
		if err := repo.InitFSRepo(repoDir, repo.Version, config.NewDefaultConfig()); err != nil {
			return err
		}
		rep, err := repo.OpenFSRepo(repoDir, repo.Version)
		if err != nil {
			return err
		}
		// The only error Close can return is that the repo has already been closed.
		defer func() { _ = rep.Close() }()

		genesisFileSource, _ := req.Options[GenesisFile].(string)
		// Writing to the repo here is messed up; this should create a genesis init function that
		// writes to the repo when invoked.
		genesisFile, err := loadGenesis(req.Context, rep, genesisFileSource)
		if err != nil {
			return err
		}

		peerKeyFile, _ := req.Options[PeerKeyFile].(string)
		initopts, err := getNodeInitOpts(peerKeyFile)
		if err != nil {
			return err
		}

		if err := node.Init(req.Context, rep, genesisFile, initopts...); err != nil {
			return err
		}

		cfg := rep.Config()
		if err := setConfigFromOptions(cfg, req.Options); err != nil {
			return err
		}
		if err := rep.ReplaceConfig(cfg); err != nil {
			return err
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(initTextEncoder),
	},
}

func setConfigFromOptions(cfg *config.Config, options cmdkit.OptMap) error {
	var err error
	if dir, ok := options[OptionSectorDir].(string); ok {
		cfg.SectorBase.RootDir = dir
	}

	if m, ok := options[WithMiner].(string); ok {
		if cfg.Mining.MinerAddress, err = address.NewFromString(m); err != nil {
			return err
		}
	}

	if autoSealIntervalSeconds, ok := options[AutoSealIntervalSeconds]; ok {
		cfg.Mining.AutoSealIntervalSeconds = autoSealIntervalSeconds.(uint)
	}

	if m, ok := options[DefaultAddress].(string); ok {
		if cfg.Wallet.DefaultAddress, err = address.NewFromString(m); err != nil {
			return err
		}
	}

	devnetTest, _ := options[DevnetStaging].(bool)
	devnetNightly, _ := options[DevnetNightly].(bool)
	devnetUser, _ := options[DevnetUser].(bool)
	if (devnetTest && devnetNightly) || (devnetTest && devnetUser) || (devnetNightly && devnetUser) {
		return fmt.Errorf(`cannot specify more than one "devnet-" option`)
	}

	// Setup devnet specific config options.
	if devnetTest || devnetNightly || devnetUser {
		cfg.Bootstrap.MinPeerThreshold = 1
		cfg.Bootstrap.Period = "10s"
	}

	// Setup devnet staging specific config options.
	if devnetTest {
		cfg.Bootstrap.Addresses = fixtures.DevnetStagingBootstrapAddrs
	}

	// Setup devnet nightly specific config options.
	if devnetNightly {
		cfg.Bootstrap.Addresses = fixtures.DevnetNightlyBootstrapAddrs
	}

	// Setup devnet user specific config options.
	if devnetUser {
		cfg.Bootstrap.Addresses = fixtures.DevnetUserBootstrapAddrs
	}

	return nil
}

func initTextEncoder(_ *cmds.Request, w io.Writer, val interface{}) error {
	_, err := fmt.Fprintf(w, val.(string))
	return err
}

func loadGenesis(ctx context.Context, rep repo.Repo, sourceName string) (consensus.GenesisInitFunc, error) {
	if sourceName == "" {
		return consensus.MakeGenesisFunc(consensus.ProofsMode(types.LiveProofsMode)), nil
	}

	source, err := openGenesisSource(sourceName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = source.Close() }()

	genesisBlk, err := extractGenesisBlock(source, rep)
	if err != nil {
		return nil, err
	}

	gif := func(cst *hamt.BasicCborIpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		return genesisBlk, err
	}

	return gif, nil

}

func getNodeInitOpts(peerKeyFile string) ([]node.InitOpt, error) {
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

	return initOpts, nil
}

func openGenesisSource(sourceName string) (io.ReadCloser, error) {
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
	return source, nil
}

func extractGenesisBlock(source io.ReadCloser, rep repo.Repo) (*block.Block, error) {
	bs := blockstore.NewBlockstore(rep.Datastore())
	ch, err := car.LoadCar(bs, source)
	if err != nil {
		return nil, err
	}

	// need to check if we are being handed a car file with a single genesis block or an entire chain.
	bsBlk, err := bs.Get(ch.Roots[0])
	if err != nil {
		return nil, err
	}
	cur, err := block.DecodeBlock(bsBlk.RawData())
	if err != nil {
		return nil, err
	}

	// the root block of the car file has parents, this file must contain a chain.
	var gensisBlk *block.Block
	if !cur.Parents.Equals(block.UndefTipSet.Key()) {
		// walk back up the chain until we hit a block with no parents, the genesis block.
		for !cur.Parents.Equals(block.UndefTipSet.Key()) {
			bsBlk, err := bs.Get(cur.Parents.ToSlice()[0])
			if err != nil {
				return nil, err
			}
			cur, err = block.DecodeBlock(bsBlk.RawData())
			if err != nil {
				return nil, err
			}
		}

		gensisBlk = cur

		logInit.Infow("initialized go-filecoin with genesis file containing partial chain", "genesisCID", gensisBlk.Cid().String(), "headCIDs", ch.Roots)
	} else {
		gensisBlk = cur
	}
	return gensisBlk, nil
}
