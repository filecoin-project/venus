package commands

import (
	"fmt"
	"io"
	"os"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/api"
)

var initCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Initialize a filecoin repo",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(WalletFile, "wallet data file: contains addresses and private keys").WithDefault(""),
		cmdkit.StringOption(WalletAddr, "address to store in nodes backend when '--walletfile' option is passed").WithDefault(""),
		cmdkit.StringOption(GenesisFile, "path of file containing archive of genesis block DAG data"),
		cmdkit.BoolOption(TestGenesis, "when set, creates a custom genesis block with pre-mined funds"),
		cmdkit.StringOption(PeerKeyFile, "path of file containing key to use for new nodes libp2p identity"),
		cmdkit.StringOption(WithMiner, "when set, creates a custom genesis block with a pre generated miner account, requires to run the daemon using dev mode (--dev)"),
		cmdkit.UintOption(AutoSealIntervalSeconds, "when set to a number > 0, configures the daemon to check for and seal any staged sectors on an interval.").WithDefault(uint(120)),
		cmdkit.BoolOption(ClusterLabWeek, "when set, populates config bootstrap addrs with the dns multiaddrs of the lab week cluster and other team week specific bootstrap parameters"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		repoDir := getRepoDir(req)
		re.Emit(fmt.Sprintf("initializing filecoin node at %s\n", repoDir)) // nolint: errcheck

		walletFile, _ := req.Options[WalletFile].(string)
		walletAddr, _ := req.Options[WalletAddr].(string)
		genesisFile, _ := req.Options[GenesisFile].(string)
		customGenesis, _ := req.Options[TestGenesis].(bool)
		peerKeyFile, _ := req.Options[PeerKeyFile].(string)
		autoSealIntervalSeconds, _ := req.Options[AutoSealIntervalSeconds].(uint)
		teamWeek, _ := req.Options[ClusterLabWeek].(bool)

		var withMiner address.Address
		if m, ok := req.Options["with-miner"].(string); ok {
			var err error
			withMiner, err = address.NewFromString(m)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		err := GetAPI(env).Daemon().Init(
			req.Context,
			api.RepoDir(repoDir),
			api.WalletFile(walletFile),
			api.WalletAddr(walletAddr),
			api.GenesisFile(genesisFile),
			api.UseCustomGenesis(customGenesis),
			api.PeerKeyFile(peerKeyFile),
			api.WithMiner(withMiner),
			api.LabWeekCluster(teamWeek),
			api.AutoSealIntervalSeconds(autoSealIntervalSeconds),
		)

		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
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
