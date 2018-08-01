package commands

import (
	"fmt"
	"io"
	"os"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/wallet"
)

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("walletfile", "wallet data file: contains addresses and private keys").WithDefault(""),
		cmdkit.StringOption("walletaddr", "address to store in nodes backend when '--walletfile' option is passed").WithDefault(""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := initRun(req, re, env); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(initTextEncoder),
	},
}

func initRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) (err error) {
	repoDir := getRepoDir(req)
	re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)) // nolint: errcheck

	if err := repo.InitFSRepo(repoDir, config.NewDefaultConfig()); err != nil {
		return err
	}

	rep, err := repo.OpenFSRepo(repoDir)
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

	walletFile, _ := req.Options["walletfile"].(string)
	walletAddr, _ := req.Options["walletaddr"].(string)
	// Used for testing
	if len(walletFile) > 1 {
		re.Emit(fmt.Sprintf("initializing filecoin node with wallet file: %s\n", walletFile)) // nolint: errcheck

		// Load all the Address their Keys into memory
		addressKeys, err := th.LoadWalletAddressAndKeysFromFile(walletFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load wallet file: %s", walletFile)
		}

		// Generate a genesis function to allocate the address funds
		var actorOps []th.GenOption
		for k, v := range addressKeys {
			actorOps = append(actorOps, th.ActorAccount(k.Address, k.Balance))

			// load an address into nodes wallet backend
			if k.Address.String() == walletAddr {
				re.Emit(fmt.Sprintf("initializing filecoin node with address: %s, balance: %s\n", k.Address.String(), k.Balance.String())) // nolint: errcheck
				if err := loadAddress(k, v, rep); err != nil {
					return err
				}
				// since `node.Init` will create an address, and since `node.Build` will
				// return `ErrNoDefaultMessageFromAddress` iff len(wallet.Addresses) > 1 and
				// wallet.defaultAddress == "" we set the default here
				rep.Config().Wallet.DefaultAddress = k.Address
			}
		}
		tif := th.MakeGenesisFunc(actorOps...)

		return node.Init(req.Context, rep, tif)
	}

	// TODO don't create the repo if this fails
	return node.Init(req.Context, rep, core.InitGenesis)
}

func loadAddress(ai th.TypesAddressInfo, ki types.KeyInfo, r repo.Repo) error {
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	if err != nil {
		return err
	}
	if err := backend.LoadAddress(ai, ki); err != nil {
		return err
	}
	return nil
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
