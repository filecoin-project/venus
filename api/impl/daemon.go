package impl

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	crypto "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	hamt "gx/ipfs/QmQZadYTDF4ud9DdK85PH2vReJRzUM9YfVW4ReB1q2m51p/go-hamt-ipld"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
	car "gx/ipfs/QmcQSyreJnxiZ1TCop3s5hjgsggpzCNjrbgqzUoQv4ywEW/go-car"
	blockstore "gx/ipfs/QmcmpX42gtDv1fz24kau4wjS9hfwWj5VexWBKgGnWzsyag/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/fixtures"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
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

// Start, starts a new daemon process.
func (nd *nodeDaemon) Start(ctx context.Context) error {
	return nd.api.node.Start(ctx)
}

// Stop, shuts down the daemon and cleans up any resources.
func (nd *nodeDaemon) Stop(ctx context.Context) error {
	nd.api.node.Stop(ctx)

	return nil
}

// Init, initializes everything needed to run a daemon, including the disk storage.
func (nd *nodeDaemon) Init(ctx context.Context, opts ...api.DaemonInitOpt) error {
	// load configuration options
	cfg := &api.DaemonInitConfig{}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			return err
		}
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

	tif := consensus.InitGenesis

	if cfg.UseCustomGenesis && cfg.GenesisFile != "" {
		return fmt.Errorf("cannot use testgenesis option and genesisfile together")
	}

	var initopts []node.InitOpt
	if cfg.PeerKeyFile != "" {
		peerKey, err := loadPeerKey(cfg.PeerKeyFile)
		if err != nil {
			return err
		}
		initopts = append(initopts, node.PeerKeyOpt(peerKey))
	}

	initopts = append(initopts, node.PerformRealProofsOpt(cfg.PerformRealProofs))
	initopts = append(initopts, node.AutoSealIntervalSecondsOpt(cfg.AutoSealIntervalSeconds))

	if cfg.WithMiner != (address.Address{}) {
		newConfig := rep.Config()
		newConfig.Mining.MinerAddress = cfg.WithMiner
		if err := rep.ReplaceConfig(newConfig); err != nil {
			return err
		}
	}

	// Setup labweek release specific config options.
	if cfg.LabWeekCluster {
		newConfig := rep.Config()
		newConfig.Bootstrap.Relays = fixtures.LabWeekRelayAddrs
		newConfig.Bootstrap.Addresses = fixtures.LabWeekBootstrapAddrs
		newConfig.Bootstrap.MinPeerThreshold = 3
		newConfig.Bootstrap.Period = "10s"
		newConfig.Swarm.Relay = true
		if err := rep.ReplaceConfig(newConfig); err != nil {
			return err
		}
	}

	// Setup enableRelayHop specific config options.
	if cfg.EnableRelayHop {
		newConfig := rep.Config()
		newConfig.Swarm.RelayHop = true
		if err := rep.ReplaceConfig(newConfig); err != nil {
			return err
		}
	}

	switch {
	case cfg.GenesisFile != "":
		// TODO: this feels a little wonky, I think the InitGenesis interface might need some tweaking
		genCid, err := LoadGenesis(rep, cfg.GenesisFile)
		if err != nil {
			return err
		}

		tif = func(cst *hamt.CborIpldStore, bs blockstore.Blockstore) (*types.Block, error) {
			var blk types.Block

			if err := cst.Get(ctx, genCid, &blk); err != nil {
				return nil, err
			}

			return &blk, nil
		}

	case cfg.UseCustomGenesis: // Used for testing
		nd.api.logger.Infof("initializing filecoin node with wallet file: %s\n", cfg.WalletFile)

		// Load all the Address their Keys into memory
		addressKeys, err := wallet.LoadWalletAddressAndKeysFromFile(cfg.WalletFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load wallet file: %s", cfg.WalletFile)
		}

		// Generate a genesis function to allocate the address funds
		var actorOps []consensus.GenOption
		for k, v := range addressKeys {
			actorOps = append(actorOps, consensus.ActorAccount(k.Address, k.Balance))

			// load an address into nodes wallet backend
			if k.Address.String() == cfg.WalletAddr {
				nd.api.logger.Infof("initializing filecoin node with address: %s, balance: %s\n", k.Address.String(), k.Balance.String())
				if err := loadAddress(k, v, rep); err != nil {
					return err
				}
				// since `node.Init` will create an address, and since `node.Build` will
				// return `ErrNoDefaultMessageFromAddress` iff len(wallet.Addresses) > 1 and
				// wallet.defaultAddress == "" we set the default here
				rep.Config().Wallet.DefaultAddress = k.Address
			}
		}
		tif = consensus.MakeGenesisFunc(actorOps...)
	}

	// TODO: don't create the repo if this fails
	return node.Init(ctx, rep, tif, initopts...)
}

func loadPeerKey(fname string) (crypto.PrivKey, error) {
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(data)
}

func loadAddress(ai wallet.TypesAddressInfo, ki types.KeyInfo, r repo.Repo) error {
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	if err != nil {
		return err
	}

	// sanity...
	a, err := ki.Address()
	if err != nil {
		return err
	}

	if a != ai.Address {
		return fmt.Errorf("mismatch of addresses: %q != %q", a, ai.Address)
	}

	if err := backend.ImportKey(&ki); err != nil {
		return err
	}
	return nil
}

// LoadGenesis gets the genesis block from a filecoin repo and a car file
func LoadGenesis(rep repo.Repo, fname string) (*cid.Cid, error) {
	fi, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fi.Close() // nolint: errcheck

	bs := blockstore.NewBlockstore(rep.Datastore())

	ch, err := car.LoadCar(bs, fi)
	if err != nil {
		return nil, err
	}

	if len(ch.Roots) != 1 {
		return nil, fmt.Errorf("expected car with only a single root")
	}

	return ch.Roots[0], nil
}
