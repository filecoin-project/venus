package commands

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds/http"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/repo"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"

	// OptionRepoDir is the name of the option for specifying the directory of the repo.
	OptionRepoDir = "repodir"

	// APIPrefix is the prefix for the http version of the api.
	APIPrefix = "/api"

	// OfflineMode tells us if we should try to connect this Filecoin node to the network
	OfflineMode = "offline"

	// ELStdout tells the daemon to write event logs to stdout.
	ELStdout = "elstdout"

	// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
	AutoSealIntervalSeconds = "auto-seal-interval-seconds"

	// SwarmListen is the multiaddr for this Filecoin node
	SwarmListen = "swarmlisten"

	// BlockTime is the duration string of the block time the daemon will
	// run with.  TODO: this should eventually be more explicitly grouped
	// with testing as we won't be able to set blocktime in production.
	BlockTime = "block-time"

	// PeerKeyFile is the path of file containing key to use for new nodes libp2p identity
	PeerKeyFile = "peerkeyfile"

	// WithMiner when set, creates a custom genesis block with a pre generated miner account, requires to run the daemon using dev mode (--dev)
	WithMiner = "with-miner"

	// TestGenesis when set, creates a custom genesis block with pre-mined funds
	TestGenesis = "testgenesis"

	// GenesisFile is the path of file containing archive of genesis block DAG data
	GenesisFile = "genesisfile"

	// WalletAddr is the address to store in nodes backend when '--walletfile' option is passed to init
	WalletAddr = "walletaddr"

	// WalletFile is the wallet data file; it contains addresses and private keys
	WalletFile = "walletfile"

	// ClusterLabWeek populates config bootstrap addrs with the dns multiaddrs of the team week cluster and other team week specific bootstrap parameters
	ClusterLabWeek = "cluster-labweek"
)

var rootCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "A decentralized storage network",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(OptionAPI, "set the api port to use"),
		cmdkit.StringOption(OptionRepoDir, "set the directory of the repo, defaults to ~/.filecoin"),
		cmds.OptionEncodingType,
		cmdkit.BoolOption("help", "Show the full command help text."),
		cmdkit.BoolOption("h", "Show a short version of the command help text."),
	},
	Subcommands: make(map[string]*cmds.Command),
}

// all top level commands. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"actor":            actorCmd,
	"address":          addrsCmd,
	"bootstrap":        bootstrapCmd,
	"chain":            chainCmd,
	"config":           configCmd,
	"client":           clientCmd,
	"daemon":           daemonCmd,
	"dag":              dagCmd,
	"id":               idCmd,
	"init":             initCmd,
	"log":              logCmd,
	"message":          msgCmd,
	"miner":            minerCmd,
	"mining":           miningCmd,
	"mpool":            mpoolCmd,
	"paych":            paymentChannelCmd,
	"ping":             pingCmd,
	"retrieval-client": retrievalClientCmd,
	"show":             showCmd,
	"swarm":            swarmCmd,
	"version":          versionCmd,
	"wallet":           walletCmd,
}

func init() {
	for k, v := range rootSubcmdsDaemon {
		rootCmd.Subcommands[k] = v
	}
}

// Run processes the arguments and stdin
func Run(args []string, stdin, stdout, stderr *os.File) (int, error) {
	return CliRun(context.Background(), rootCmd, args, stdin, stdout, stderr, buildEnv, makeExecutor)
}

func buildEnv(ctx context.Context, req *cmds.Request) (cmds.Environment, error) {
	return &Env{ctx: ctx, api: impl.New(nil)}, nil
}

type executor struct {
	api  string
	exec cmds.Executor
}

func (e *executor) Execute(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	if e.api == "" {
		return e.exec.Execute(req, re, env)
	}

	client := cmdhttp.NewClient(e.api, cmdhttp.ClientWithAPIPrefix(APIPrefix))

	res, err := client.Send(req)
	if err != nil {
		if isConnectionRefused(err) {
			re.SetError("Connection Refused. Is the daemon running?", cmdkit.ErrFatal)
			return nil
		}
		re.SetError(err, cmdkit.ErrFatal)
		return nil
	}

	wait := make(chan struct{})
	// copy received result into cli emitter
	go func() {
		err := cmds.Copy(re, res)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal|cmdkit.ErrFatal)
		}
		close(wait)
	}()

	<-wait
	return nil
}

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	isDaemonRequired := requiresDaemon(req)
	var api string
	if isDaemonRequired {
		var err error
		api, err = getAPIAddress(req)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get API address")
		}
	}

	if api == "" && isDaemonRequired {
		return nil, ErrMissingDaemon
	}

	return &executor{
		api:  api,
		exec: cmds.NewExecutor(rootCmd),
	}, nil
}

func getAPIAddress(req *cmds.Request) (string, error) {
	var out string
	// second highest precedence is env vars.
	if envapi := os.Getenv("FIL_API"); envapi != "" {
		out = envapi
	}

	// first highest precedence is cmd flag.
	if apiAddress, ok := req.Options[OptionAPI].(string); ok && apiAddress != "" {
		out = apiAddress
	}

	// we will read the api file if no other option is given.
	if len(out) == 0 {
		apiFilePath, err := homedir.Expand(filepath.Join(filepath.Clean(getRepoDir(req)), repo.APIFile))
		if err != nil {
			return "", errors.Wrap(err, "failed to read api file")
		}

		out, err = repo.APIAddrFromFile(apiFilePath)
		if err != nil {
			return "", err
		}

	}

	maddr, err := ma.NewMultiaddr(out)
	if err != nil {
		return "", err
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return "", err
	}

	return host, nil
}

func requiresDaemon(req *cmds.Request) bool {
	if req.Command == daemonCmd {
		return false
	}

	if req.Command == initCmd {
		return false
	}

	return true
}

func isConnectionRefused(err error) bool {
	urlErr, ok := err.(*url.Error)
	if !ok {
		return false
	}

	opErr, ok := urlErr.Err.(*net.OpError)
	if !ok {
		return false
	}

	syscallErr, ok := opErr.Err.(*os.SyscallError)
	if !ok {
		return false
	}
	return syscallErr.Err == syscall.ECONNREFUSED
}
