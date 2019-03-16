package commands

import (
	"fmt"
	"io"
	"os"

	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	cmds "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
)

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

		return GetAPI(env).Daemon().Init(
			req.Context,
			api.RepoDir(repoDir),
			api.GenesisFile(genesisFile),
			api.PeerKeyFile(peerKeyFile),
			api.WithMiner(withMiner),
			api.DevnetTest(devnetTest),
			api.DevnetNightly(devnetNightly),
			api.DevnetUser(devnetUser),
			api.AutoSealIntervalSeconds(autoSealIntervalSeconds),
			api.DefaultAddress(defaultAddress),
		)
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
