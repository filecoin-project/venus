package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-car"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/node"
	"github.com/filecoin-project/venus/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/config"
	"github.com/filecoin-project/venus/internal/pkg/genesis"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
)

var logInit = logging.Logger("commands/init")

var initCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmds.Option{
		cmds.StringOption(GenesisFile, "path of file or HTTP(S) URL containing archive of genesis block DAG data"),
		cmds.StringOption(PeerKeyFile, "path of file containing key to use for new node's libp2p identity"),
		cmds.StringOption(WalletKeyFile, "path of file containing keys to import into the wallet on initialization"),
		cmds.StringOption(Network, "when set, populates config with network specific parameters"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err := paths.GetRepoPath(repoDir)
		if err != nil {
			return err
		}

		if err := re.Emit(repoDir); err != nil {
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
		gif, err := loadGenesis(req.Context, rep, genesisFileSource)
		if err != nil {
			return err
		}

		peerKeyFile, _ := req.Options[PeerKeyFile].(string)
		walletKeyFile, _ := req.Options[WalletKeyFile].(string)
		initopts, err := getNodeInitOpts(peerKeyFile, walletKeyFile)
		if err != nil {
			return err
		}

		cfg := rep.Config()
		if err := setConfigFromOptions(cfg, req.Options); err != nil {
			logInit.Errorf("Error setting config %s", err)
			return err
		}

		if err := rep.ReplaceConfig(cfg); err != nil {
			logInit.Errorf("Error replacing config %s", err)
			return err
		}

		logInit.Info("Initializing node")
		if err := node.Init(req.Context, rep, gif, initopts...); err != nil {
			logInit.Errorf("Error initializing node %s", err)
			return err
		}

		return nil
	},
}

func setConfigFromOptions(cfg *config.Config, options cmds.OptMap) error {
	// Setup devnet specific config options.
	netName, _ := options[Network].(string)
	var netcfg *networks.NetworkConf
	if netName == "mainnet" {
		netcfg = networks.Mainnet()
	} else if netName == "testnetnet" {
		netcfg = networks.Testnet()
	} else if netName == "integrationnet" {
		netcfg = networks.IntegrationNet()
	} else if netName != "" {
		return fmt.Errorf("unknown network name %s", netName)
	}

	if netcfg != nil {
		cfg.Bootstrap = &netcfg.Bootstrap
		cfg.NetworkParams = &netcfg.Network
	}

	return nil
}

// helper type to implement plumbing subset
type setWrapper struct {
	cfg *config.Config
}

func (w *setWrapper) ConfigSet(dottedKey string, jsonString string) error {
	return w.cfg.Set(dottedKey, jsonString)
}

func loadGenesis(ctx context.Context, rep repo.Repo, sourceName string) (genesis.InitFunc, error) {
	if sourceName == "" {
		return gengen.MakeGenesisFunc(), nil
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

	gif := func(cst cbor.IpldStore, bs blockstore.Blockstore) (*block.Block, error) {
		return genesisBlk, err
	}

	return gif, nil

}

func getNodeInitOpts(peerKeyFile string, walletKeyFile string) ([]node.InitOpt, error) {
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

	if walletKeyFile != "" {
		f, err := os.Open(walletKeyFile)
		if err != nil {
			return nil, err
		}

		var wir *WalletSerializeResult
		if err := json.NewDecoder(f).Decode(&wir); err != nil {
			return nil, err
		}

		if len(wir.KeyInfo) > 0 {
			initOpts = append(initOpts, node.DefaultKeyOpt(wir.KeyInfo[0]))
		}

		for _, k := range wir.KeyInfo[1:] {
			initOpts = append(initOpts, node.ImportKeyOpt(k))
		}
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

	//parent also have parents
	// the root block of the car file has parents, this file must contain a chain.
	/*	var gensisBlk *block.Block
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

			logInit.Infow("initialized venus with genesis file containing partial chain", "genesisCID", gensisBlk.Cid().String(), "headCIDs", ch.Roots)
		} else {
			gensisBlk = cur
		}*/
	return cur, nil
}
