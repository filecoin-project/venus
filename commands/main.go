package commands

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZcLBXKaFe8ND5YHPkJRAwmhJGrVsi1JqDZNyJ4nRK5Mj/go-multiaddr-net"
	"gx/ipfs/QmdcULN1WCzgoQmcCaUAmEhwcxHYsDrbZ2LvRJKCL8dMrK/go-homedir"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"
	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds/cli"
	cmdhttp "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds/http"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
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

	// SwarmAddress is the multiaddr for this Filecoin node
	SwarmAddress = "swarmlisten"

	// SwarmPublicRelayAddress is a public address that the filecoin node
	// will listen on if it is operating as a relay.  We use this to specify
	// the public ip:port of a relay node that is sitting behind a static
	// NAT mapping.
	SwarmPublicRelayAddress = "swarmrelaypublic"

	// BlockTime is the duration string of the block time the daemon will
	// run with.  TODO: this should eventually be more explicitly grouped
	// with testing as we won't be able to set blocktime in production.
	BlockTime = "block-time"

	// PeerKeyFile is the path of file containing key to use for new nodes libp2p identity
	PeerKeyFile = "peerkeyfile"

	// WithMiner when set, creates a custom genesis block with a pre generated miner account, requires to run the daemon using dev mode (--dev)
	WithMiner = "with-miner"

	// DefaultAddress when set, sets the daemons's default address to the provided address
	DefaultAddress = "default-address"

	// GenesisFile is the path of file containing archive of genesis block DAG data
	GenesisFile = "genesisfile"

	// DevnetTest populates config bootstrap addrs with the dns multiaddrs of the test devnet and other test devnet specific bootstrap parameters
	DevnetTest = "devnet-test"

	// DevnetNightly populates config bootstrap addrs with the dns multiaddrs of the nightly devnet and other nightly devnet specific bootstrap parameters
	DevnetNightly = "devnet-nightly"

	// DevnetUser populates config bootstrap addrs with the dns multiaddrs of the user devnet and other user devnet specific bootstrap parameters
	DevnetUser = "devnet-user"

	// IsRelay when set causes the the daemon to provide libp2p relay
	// services allowing other filecoin nodes behind NATs to talk directly.
	IsRelay = "is-relay"
)

// command object for the local cli
var rootCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "A decentralized storage network",
		Subcommands: `
START RUNNING FILECOIN
  go-filecoin init                   - Initialize a filecoin repo
  go-filecoin config <key> [<value>] - Get and set filecoin config values
  go-filecoin daemon                 - Start a long-running daemon process
  go-filecoin wallet                 - Manage your filecoin wallets
  go-filecoin address                - Interact with addresses

STORE AND RETRIEVE DATA
  go-filecoin client                 - Make deals, store data, retrieve data
  go-filecoin retrieval-client       - Manage retrieval client operations

MINE
  go-filecoin miner                  - Manage a single miner actor
  go-filecoin mining                 - Manage all mining operations for a node

VIEW DATA STRUCTURES
  go-filecoin chain                  - Inspect the filecoin blockchain
  go-filecoin dag                    - Interact with IPLD DAG objects
  go-filecoin show                   - Get human-readable representations of filecoin objects

NETWORK COMMANDS
  go-filecoin bootstrap              - Interact with bootstrap addresses
  go-filecoin dht                    - Interact with the dht
  go-filecoin id                     - Show info about the network peers
  go-filecoin ping <peer ID>...      - Send echo request packets to p2p network members
  go-filecoin swarm                  - Interact with the swarm
  go-filecoin stats                  - Monitor statistics on your network usage

ACTOR COMMANDS
  go-filecoin actor                  - Interact with actors. Actors are built-in smart contracts.
  go-filecoin paych                  - Payment channel operations

MESSAGE COMMANDS
  go-filecoin message                - Manage messages
  go-filecoin mpool                  - Manage the message pool

TOOL COMMANDS
  go-filecoin log                    - Interact with the daemon event log output.
  go-filecoin version                - Show go-filecoin version information
`,
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

// command object for the daemon
var rootCmdDaemon = &cmds.Command{
	Subcommands: make(map[string]*cmds.Command),
}

// all top level commands, not available to daemon
var rootSubcmdsLocal = map[string]*cmds.Command{
	"daemon": daemonCmd,
	"init":   initCmd,
}

// all top level commands, available on daemon. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"actor":            actorCmd,
	"address":          addrsCmd,
	"bootstrap":        bootstrapCmd,
	"chain":            chainCmd,
	"config":           configCmd,
	"client":           clientCmd,
	"dag":              dagCmd,
	"dht":              dhtCmd,
	"id":               idCmd,
	"log":              logCmd,
	"message":          msgCmd,
	"miner":            minerCmd,
	"mining":           miningCmd,
	"mpool":            mpoolCmd,
	"outbox":           outboxCmd,
	"paych":            paymentChannelCmd,
	"ping":             pingCmd,
	"retrieval-client": retrievalClientCmd,
	"show":             showCmd,
	"stats":            statsCmd,
	"swarm":            swarmCmd,
	"version":          versionCmd,
	"wallet":           walletCmd,
}

func init() {
	for k, v := range rootSubcmdsLocal {
		rootCmd.Subcommands[k] = v
	}

	for k, v := range rootSubcmdsDaemon {
		rootCmd.Subcommands[k] = v
		rootCmdDaemon.Subcommands[k] = v
	}
}

// Run processes the arguments and stdin
func Run(args []string, stdin, stdout, stderr *os.File) (int, error) {
	err := cli.Run(context.Background(), rootCmd, args, stdin, stdout, stderr, buildEnv, makeExecutor)
	if err == nil {
		return 0, nil
	}
	if exerr, ok := err.(cli.ExitError); ok {
		return int(exerr), nil
	}
	return 1, err
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
			return cmdkit.Errorf(cmdkit.ErrFatal, "Connection Refused. Is the daemon running?")
		}
		return cmdkit.Errorf(cmdkit.ErrFatal, err.Error())
	}

	// copy received result into cli emitter
	err = cmds.Copy(re, res)
	if err != nil {
		return cmdkit.Errorf(cmdkit.ErrFatal|cmdkit.ErrNormal, err.Error())
	}
	return nil
}

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	isDaemonRequired := requiresDaemon(req)
	var api string
	if isDaemonRequired {
		var err error
		api, err = getAPIAddress(req)
		if err != nil {
			return nil, err
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
	var rawAddr string
	// second highest precedence is env vars.
	if envapi := os.Getenv("FIL_API"); envapi != "" {
		rawAddr = envapi
	}

	// first highest precedence is cmd flag.
	if apiAddress, ok := req.Options[OptionAPI].(string); ok && apiAddress != "" {
		rawAddr = apiAddress
	}

	// we will read the api file if no other option is given.
	if len(rawAddr) == 0 {
		rawPath := filepath.Join(filepath.Clean(getRepoDir(req)), repo.APIFile)
		apiFilePath, err := homedir.Expand(rawPath)
		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("can't resolve local repo path %s", rawPath))
		}

		rawAddr, err = repo.APIAddrFromFile(apiFilePath)
		if err != nil {
			return "", errors.Wrap(err, "can't find API endpoint address in environment, command-line, or local repo (is the daemon running?)")
		}

	}

	maddr, err := ma.NewMultiaddr(rawAddr)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to convert API endpoint address %s to a multiaddr", rawAddr))
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to dial API endpoint address %s", maddr))
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

var priceOption = cmdkit.StringOption("gas-price", "Price (FIL e.g. 0.00013) to pay for each GasUnits consumed mining this message")
var limitOption = cmdkit.Uint64Option("gas-limit", "Maximum number of GasUnits this message is allowed to consume")
var previewOption = cmdkit.BoolOption("preview", "Preview the Gas cost of this command without actually executing it")

func parseGasOptions(req *cmds.Request) (types.AttoFIL, types.GasUnits, bool, error) {
	priceOption := req.Options["gas-price"]
	if priceOption == nil {
		return types.AttoFIL{}, types.NewGasUnits(0), false, errors.New("price option is required")
	}

	price, ok := types.NewAttoFILFromFILString(priceOption.(string))
	if !ok {
		return types.AttoFIL{}, types.NewGasUnits(0), false, errors.New("invalid gas price (specify FIL as a decimal number)")
	}

	limitOption := req.Options["gas-limit"]
	if limitOption == nil {
		return types.AttoFIL{}, types.NewGasUnits(0), false, errors.New("limit option is required")
	}

	gasLimitInt, ok := limitOption.(uint64)
	if !ok {
		msg := fmt.Sprintf("invalid gas limit: %s", limitOption)
		return types.AttoFIL{}, types.NewGasUnits(0), false, errors.New(msg)
	}

	preview, _ := req.Options["preview"].(bool)

	return *price, types.NewGasUnits(gasLimitInt), preview, nil
}
