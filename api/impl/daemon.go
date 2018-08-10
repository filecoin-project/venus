package impl

import (
	"context"
	"fmt"
	"os"

	car "gx/ipfs/QmUe7hFx8ACivDWe1pF6X2ZTihGfeXppMc1aPjNBJ8cCHv/go-car"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	hamt "gx/ipfs/QmXJkSRxXHeAGmQJENct16anrKZHNECbmUoC7hMuCjLni6/go-hamt-ipld"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	blockstore "gx/ipfs/QmcD7SqfyQyA91TZUQ7VPRYbGarxmY7EsQewVYMuN5LNSv/go-ipfs-blockstore"
	"gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"

	"github.com/filecoin-project/go-filecoin/api"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

type nodeDaemon struct {
	api *nodeAPI
}

func newNodeDaemon(api *nodeAPI) *nodeDaemon {
	return &nodeDaemon{api: api}
}

// Start, starts a new daemon process.
func (nd *nodeDaemon) Start(ctx context.Context) error {
	return nd.api.node.Start()
}

// Stop, shuts down the daemon and cleans up any resources.
func (nd *nodeDaemon) Stop(ctx context.Context) error {
	nd.api.node.Stop()

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

	tif := core.InitGenesis

	if cfg.UseCustomGenesis && cfg.GenesisFile != "" {
		return fmt.Errorf("cannot use testgenesis option and genesisfile together")
	}

	switch {
	case cfg.GenesisFile != "":
		// TODO: this feels a little wonky, I think the InitGenesis interface might need some tweaking
		genCid, err := loadGenesis(ctx, rep, cfg.GenesisFile)
		if err != nil {
			return err
		}

		tif = func(cst *hamt.CborIpldStore, ds datastore.Datastore) (*types.Block, error) {
			var blk types.Block

			if err := cst.Get(ctx, genCid, &blk); err != nil {
				return nil, err
			}

			return &blk, nil
		}

	case cfg.UseCustomGenesis: // Used for testing
		nd.api.logger.Infof("initializing filecoin node with wallet file: %s\n", cfg.WalletFile)

		// Load all the Address their Keys into memory
		addressKeys, err := testhelpers.LoadWalletAddressAndKeysFromFile(cfg.WalletFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load wallet file: %s", cfg.WalletFile)
		}

		// Generate a genesis function to allocate the address funds
		var actorOps []testhelpers.GenOption
		for k, v := range addressKeys {
			actorOps = append(actorOps, testhelpers.ActorAccount(k.Address, k.Balance))

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
		tif = testhelpers.MakeGenesisFunc(actorOps...)
	}

	// TODO: don't create the repo if this fails
	return node.Init(ctx, rep, tif)
}

func loadAddress(ai testhelpers.TypesAddressInfo, ki types.KeyInfo, r repo.Repo) error {
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

func loadGenesis(ctx context.Context, rep repo.Repo, fname string) (*cid.Cid, error) {
	fi, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fi.Close() // nolint: errcheck

	bs := blockstore.NewBlockstore(rep.Datastore())

	return car.LoadCar(ctx, bs, fi)
}
