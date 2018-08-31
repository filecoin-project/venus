package commands

import (
	"context"
	"net"
	"os"
	"path/filepath"

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
	// MockMineMode replaces the normal chain weight and power table
	// computations with simple computations that don't require the tip of
	// a chain to hold a pointer to a valid state root. This mode exists
	// because many daemon tests rely on mining and block processing to
	// extend the chain and process messages without setting up a storage
	// market.
	//
	// TODO: This is a TEMPORARY WORKAROUND. Ideally similar functionality
	// will be accomplished by running a node with Proof-Of-Bob consensus
	// during tests.  Alternatively we could come up with a canonical set
	// of testing genesii and allow the CLI to take in custom gensis blocks.
	MockMineMode = "mock-mine"
	// SwarmListen is the multiaddr for this Filecoin node
	SwarmListen = "swarmlisten"
	// BlockTime is the duration string of the block time the daemon will
	// run with.  TODO: this should eventually be more explicitly grouped
	// with testing as we won't be able to set blocktime in production.
	BlockTime = "block-time"
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
	"actor":     actorCmd,
	"address":   addrsCmd,
	"bootstrap": bootstrapCmd,
	"chain":     chainCmd,
	"config":    configCmd,
	"client":    clientCmd,
	"daemon":    daemonCmd,
	"dag":       dagCmd,
	"id":        idCmd,
	"init":      initCmd,
	"log":       logCmd,
	"message":   msgCmd,
	"miner":     minerCmd,
	"mining":    miningCmd,
	"mpool":     mpoolCmd,
	"orderbook": orderbookCmd,
	"paych":     paymentChannelCmd,
	"ping":      pingCmd,
	"show":      showCmd,
	"swarm":     swarmCmd,
	"version":   versionCmd,
	"wallet":    walletCmd,
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
	api     string
	running bool
	exec    cmds.Executor
}

func (e *executor) Execute(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	if !e.running {
		return e.exec.Execute(req, re, env)
	}

	client := cmdhttp.NewClient(e.api, cmdhttp.ClientWithAPIPrefix(APIPrefix))

	res, err := client.Send(req)
	if err != nil {
		return err
	}
	// send request to server
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

	isDaemonRunning, err := daemonRunning(api)
	if err != nil {
		return nil, err
	}

	if isDaemonRunning && req.Command == daemonCmd {
		return nil, ErrAlreadyRunning
	}

	if !isDaemonRunning && isDaemonRequired {
		return nil, ErrMissingDaemon
	}

	return &executor{
		api:     api,
		exec:    cmds.NewExecutor(rootCmd),
		running: isDaemonRunning,
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

func daemonRunning(api string) (bool, error) {
	// TODO: use lockfile once implemented
	// for now we just check if the port is available

	ln, err := net.Listen("tcp", api)
	if err != nil {
		return true, nil
	}

	if err := ln.Close(); err != nil {
		return false, err
	}

	return false, nil
}
