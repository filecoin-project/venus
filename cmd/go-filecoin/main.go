package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net" //nolint
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/venus/internal/pkg/repo"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"

	// OptionRepoDir is the name of the option for specifying the directory of the repo.
	OptionRepoDir = "repodir"

	// OptionSectorDir is the name of the option for specifying the directory into which staged and sealed sectors will be written.
	//OptionSectorDir = "sectordir"

	// OptionPresealedSectorDir is the name of the option for specifying the directory from which presealed sectors should be pulled when initializing.
	//OptionPresealedSectorDir = "presealed-sectordir"

	// OptionDrandConfigAddr is the init option for configuring drand to a given network address at init time
	OptionDrandConfigAddr = "drand-config-addr"

	// APIPrefix is the prefix for the http version of the api.
	APIPrefix = "/api"

	// OfflineMode tells us if we should try to connect this Filecoin node to the network
	OfflineMode = "offline"

	//  MODE start venus in lite mode
	LiteNode = "lite"

	// ELStdout tells the daemon to write event logs to stdout.
	ELStdout = "elstdout"

	// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
	//AutoSealIntervalSeconds = "auto-seal-interval-seconds"

	// SwarmAddress is the multiaddr for this Filecoin node
	SwarmAddress = "swarmlisten"

	// SwarmPublicRelayAddress is a public address that the filecoin node
	// will listen on if it is operating as a relay.  We use this to specify
	// the public ip:port of a relay node that is sitting behind a static
	// NAT mapping.
	SwarmPublicRelayAddress = "swarmrelaypublic"

	// PropagationDelay is the duration the miner will wait for blocks to arrive before attempting to mine a new one
	//PropagationDelay = "prop-delay"

	// PeerKeyFile is the path of file containing key to use for new nodes libp2p identity
	PeerKeyFile = "peerkeyfile"

	// WalletKeyFile is the path of file containing wallet keys that may be imported on initialization
	WalletKeyFile = "wallet-keyfile"

	// MinerActorAddress when set, sets the daemons's miner address to the provided address
	//MinerActorAddress = "miner-actor-address"

	// GenesisFile is the path of file containing archive of genesis block DAG data
	GenesisFile = "genesisfile"

	// Network populates config with network-specific parameters for a known network (e.g. testnet2)
	Network = "network"

	// IsRelay when set causes the the daemon to provide libp2p relay
	// services allowing other filecoin nodes behind NATs to talk directly.
	IsRelay = "is-relay"

	Size = "size"
)

func init() {
	// add pretty json as an encoding type
	cmds.Encoders["pretty-json"] = func(req *cmds.Request) func(io.Writer) cmds.Encoder {
		return func(w io.Writer) cmds.Encoder {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "\t")
			return enc
		}
	}
}

// command object for the local cli
var RootCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "A decentralized storage network",
		Subcommands: `
START RUNNING FILECOIN
  venus init                   - Initialize a filecoin repo
  venus config <key> [<value>] - Get and set filecoin config values
  venus daemon                 - Start a long-running daemon process
  venus wallet                 - Manage your filecoin wallets
  venus address                - Interact with addresses

VIEW DATA STRUCTURES
  venus chain                  - Inspect the filecoin blockchain
  venus dag                    - Interact with IPLD DAG objects
  venus show                   - Get human-readable representations of filecoin objects

NETWORK COMMANDS
  venus bootstrap              - Interact with bootstrap addresses
  venus dht                    - Interact with the dht
  venus id                     - Show info about the network peers
  venus ping <peer ID>...      - Send echo request packets to p2p network members
  venus swarm                  - Interact with the swarm
  venus stats                  - Monitor statistics on your network usage
  venus drand random           - retrieve drand randomness

ACTOR COMMANDS
  venus actor                  - Interact with actors. Actors are built-in smart contracts
  venus paych                  - Payment channel operations

MESSAGE COMMANDS
  venus message                - Manage messages
  venus mpool                  - Manage the message pool
  venus outbox                 - Manage the outbound message queue

TOOL COMMANDS
  venus inspect                - Show info about the venus node
  venus leb128                 - Leb128 cli encode/decode
  venus log                    - Interact with the daemon event log output
  venus protocol               - Show protocol parameter details
  venus version                - Show venus version information
`,
	},
	Options: []cmds.Option{
		cmds.StringOption(OptionAPI, "set the api port to use"),
		cmds.StringOption(OptionRepoDir, "set the repo directory, defaults to ~/.filecoin/repo"),
		cmds.StringOption(cmds.EncLong, cmds.EncShort, "The encoding type the output should be encoded with (pretty-json or json)").WithDefault("pretty-json"),
		cmds.BoolOption("help", "Show the full command help text."),
		cmds.BoolOption("h", "Show a short version of the command help text."),
	},
	Subcommands: make(map[string]*cmds.Command),
}

// command object for the daemon
var rootCmdDaemon = &cmds.Command{
	Subcommands: make(map[string]*cmds.Command),
}

// all top level commands, not available to daemon
var rootSubcmdsLocal = map[string]*cmds.Command{
	"daemon":  daemonCmd,
	"fetch":   fetchCmd,
	"init":    initCmd,
	"import":  importCmd,
	"version": versionCmd,
	"leb128":  leb128Cmd,
}

// all top level commands, available on daemon. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"actor":     actorCmd,
	"address":   addrsCmd,
	"bootstrap": bootstrapCmd,
	"chain":     chainCmd,
	"config":    configCmd,
	//"client":           clientCmd,
	"drand": drandCmd,
	"dag":   dagCmd,
	//"deals":            dealsCmd,
	"dht":     dhtCmd,
	"id":      idCmd,
	"inspect": inspectCmd,
	"leb128":  leb128Cmd,
	"log":     logCmd,
	"message": msgCmd,
	//"miner":            minerCmd,
	//"mining":           miningCmd,
	"mpool":    mpoolCmd,
	"outbox":   outboxCmd,
	"protocol": protocolCmd,
	"show":     showCmd,
	"stats":    statsCmd,
	"swarm":    swarmCmd,
	"wallet":   walletCmd,
	"version":  versionCmd,
}

func init() {
	for k, v := range rootSubcmdsLocal {
		RootCmd.Subcommands[k] = v
	}

	for k, v := range rootSubcmdsDaemon {
		RootCmd.Subcommands[k] = v
		rootCmdDaemon.Subcommands[k] = v
	}
}

// Run processes the arguments and stdin
func Run(ctx context.Context, args []string, stdin, stdout, stderr *os.File) (int, error) {
	err := cli.Run(ctx, RootCmd, args, stdin, stdout, stderr, buildEnv, makeExecutor)
	if err == nil {
		return 0, nil
	}
	if exerr, ok := err.(cli.ExitError); ok {
		return int(exerr), nil
	}
	return 1, err
}

func buildEnv(ctx context.Context, _ *cmds.Request) (cmds.Environment, error) {
	return NewClientEnv(ctx), nil
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

	return client.Execute(req, re, env)
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
		exec: cmds.NewExecutor(RootCmd),
	}, nil
}

func getAPIAddress(req *cmds.Request) (string, error) {
	var rawAddr string
	var err error
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
		repoDir, _ := req.Options[OptionRepoDir].(string)
		repoDir, err = paths.GetRepoPath(repoDir)
		if err != nil {
			return "", err
		}
		rawAddr, err = repo.APIAddrFromRepoPath(repoDir)
		if err != nil {
			return "", errors.Wrap(err, "can't find API endpoint address in environment, command-line, or local repo (is the daemon running?)")
		}
	}

	maddr, err := ma.NewMultiaddr(rawAddr)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to convert API endpoint address %s to a multiaddr", rawAddr))
	}

	_, host, err := manet.DialArgs(maddr) //nolint
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("unable to dial API endpoint address %s", maddr))
	}

	return host, nil
}

func requiresDaemon(req *cmds.Request) bool {
	for cmd := range rootSubcmdsLocal {
		if len(req.Path) > 0 && req.Path[0] == cmd {
			return false
		}
	}
	return true
}

var feecapOption = cmds.StringOption("gas-feecap", "Price (FIL e.g. 0.00013) to pay for each GasUnit consumed mining this message")
var premiumOption = cmds.StringOption("gas-premium", "Price (FIL e.g. 0.00013) to pay for each GasUnit consumed mining this message")
var limitOption = cmds.Int64Option("gas-limit", "Maximum GasUnits this message is allowed to consume")
var previewOption = cmds.BoolOption("preview", "Preview the Gas cost of this command without actually executing it")

func parseGasOptions(req *cmds.Request) (types.AttoFIL, types.AttoFIL, types.Unit, bool, error) {
	feecapOption := req.Options["gas-feecap"]
	if feecapOption == nil {
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.Zero, false, errors.New("gas-feecap option is required")
	}

	premiumOption := req.Options["gas-premium"]
	if feecapOption == nil {
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.Zero, false, errors.New("gas-premium option is required")
	}

	feecap, ok := types.NewAttoFILFromFILString(feecapOption.(string))
	if !ok {
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.NewGas(0), false, errors.New("invalid gas price (specify FIL as a decimal number)")
	}
	premium, ok := types.NewAttoFILFromFILString(premiumOption.(string))
	if !ok {
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.NewGas(0), false, errors.New("invalid gas price (specify FIL as a decimal number)")
	}

	limitOption := req.Options["gas-limit"]
	if limitOption == nil {
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.NewGas(0), false, errors.New("gas-limit option is required")
	}

	gasLimitInt, ok := limitOption.(int64)
	if !ok {
		msg := fmt.Sprintf("invalid gas limit: %s", limitOption)
		return types.ZeroAttoFIL, types.ZeroAttoFIL, types.NewGas(0), false, errors.New(msg)
	}

	preview, _ := req.Options["preview"].(bool)

	return feecap, premium, types.NewGas(gasLimitInt), preview, nil
}
