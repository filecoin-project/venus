package commands

import (
	"fmt"
	"io"
	"os"
	"strconv"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	errors "gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

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
		cmdkit.StringOption("genesisfil", "filecoin allocated to each address when '--walletfile' option is passed").WithDefault("10000000"),
	},
	Run: initRun,
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
	if len(walletFile) > 1 {
		var tif core.GenesisInitFunc
		re.Emit(fmt.Sprintf("initializing filecoin node with wallet file: %s\n", walletFile)) // nolint: errcheck

		nodeAddrs, err := loadAddresses(walletFile, rep)
		if err != nil {
			return errors.Wrapf(err, "failed to load wallet file: %s", walletFile)
		}

		// since `node.Init` will create an address, and since `node.Build` will
		// return `ErrNoDefaultMessageFromAddress` iff len(wallet.Addresses) > 1 and
		// wallet.defaultAddress == "" we set the default here
		rep.Config().Wallet.DefaultAddress = nodeAddrs[0]

		genesisFil, _ := req.Options["genesisfil"].(string)
		genFil, err := strconv.ParseUint(genesisFil, 10, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse genesisfil amount")
		}
		var actorOps []th.GenOption
		for i := range nodeAddrs {
			actorOps = append(actorOps, th.ActorAccount(nodeAddrs[i], types.NewAttoFILFromFIL(genFil)))
		}
		tif = th.MakeGenesisFunc(actorOps...)
		return node.Init(req.Context, rep, tif)
	}

	// TODO don't create the repo if this fails
	return node.Init(req.Context, rep, core.InitGenesis)
}

func loadAddresses(file string, r repo.Repo) ([]types.Address, error) {
	backend, err := wallet.NewDSBackend(r.WalletDatastore())
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up wallet backend")
	}

	err = backend.LoadFromFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	if len(backend.Addresses()) == 0 {
		return nil, errors.New("wallet file did not contain any addresses")
	}

	return backend.Addresses(), nil
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
