package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	fbig "github.com/filecoin-project/go-state-types/big"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/types"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"

	OptionToken = "token"
	// OptionRepoDir is the name of the option for specifying the directory of the repo.
	OptionRepoDir = "repodir"

	// OptionSectorDir is the name of the option for specifying the directory into which staged and sealed sectors will be written.
	//OptionSectorDir = "sectordir"

	// OptionPresealedSectorDir is the name of the option for specifying the directory from which presealed sectors should be pulled when initializing.
	//OptionPresealedSectorDir = "presealed-sectordir"

	// OptionDrandConfigAddr is the init option for configuring drand to a given network address at init time
	OptionDrandConfigAddr = "drand-config-addr"

	// offlineMode tells us if we should try to connect this Filecoin node to the network
	OfflineMode = "offline"

	// ELStdout tells the daemon to write event logs to stdout.
	ELStdout = "elstdout"

	// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
	//AutoSealIntervalSeconds = "auto-seal-interval-seconds"

	// SwarmAddress is the multiaddr for this Filecoin node
	SwarmAddress = "swarmlisten"

	// SwarmPublicRelayAddress is a public address that the venus node
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

	ImportSnapshot = "import-snapshot"

	// wallet password
	Password = "password"

	AuthServiceURL = "authURL"
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
  venus config <key> [<value>] - Get and set filecoin config values
  venus daemon                 - Start a long-running daemon process
  venus wallet                 - Manage your filecoin wallets
  venus msig                   - Interact with a multisig wallet

VIEW DATA STRUCTURES
  venus chain                  - Inspect the filecoin blockchain
  venus sync 				   - Inspect the filecoin Sync
  venus dag                    - Interact with IPLD DAG objects
  venus show                   - Get human-readable representations of filecoin objects

NETWORK COMMANDS
  venus swarm                  - Interact with the swarm
  venus drand                  - retrieve drand randomness

MESSAGE COMMANDS
  venus send                   - Send message
  venus mpool                  - Manage the message pool

State COMMANDS
  venus wait-msg               - Wait for a message to appear on chain
  venus search-msg             - Search to see whether a message has appeared on chain
  venus power                  - Query network or miner power
  venus sectors                - Query the sector set of a miner
  venus active-sectors         - Query the active sector set of a miner
  venus sector                 - Get miner sector info
  venus get-actor              - Print actor information
  venus lookup                 - Find corresponding ID address
  venus sector-size            - Look up miners sector size
  venus get-deal               - View on-chain deal info
  venus miner-info             - Retrieve miner information
  venus network-version        - MReturns the network version
  venus list-actor             - list all actors

Paych COMMANDS 
  venus paych                  - Manage payment channels

TOOL COMMANDS
  venus inspect                - Show info about the venus node
  venus leb128                 - Leb128 cli encode/decode
  venus log                    - Interact with the daemon event log output
  venus protocol               - Show protocol parameter details
  venus version                - Show venus version information
  venus seed                   - Seal sectors for genesis miner
  venus fetch-params           - Fetch proving parameters
`,
	},
	Options: []cmds.Option{
		cmds.StringsOption(OptionToken, "set the auth token to use"),
		cmds.StringOption(OptionAPI, "set the api port to use"),
		cmds.StringOption(OptionRepoDir, "set the repo directory, defaults to ~/.venus/repo"),
		cmds.StringOption(cmds.EncLong, cmds.EncShort, "The encoding type the output should be encoded with (pretty-json or json)").WithDefault("pretty-json"),
		cmds.BoolOption("help", "Show the full command help text."),
		cmds.BoolOption("h", "Show a short version of the command help text."),
	},
	Subcommands: make(map[string]*cmds.Command),
}

// command object for the daemon
var RootCmdDaemon = &cmds.Command{
	Subcommands: make(map[string]*cmds.Command),
}

// all top level commands, not available to daemon
var rootSubcmdsLocal = map[string]*cmds.Command{
	"daemon":  daemonCmd,
	"fetch":   fetchCmd,
	"version": versionCmd,
	"leb128":  leb128Cmd,
	"seed":    seedCmd,
}

// all top level commands, available on daemon. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"chain":    chainCmd,
	"sync":     syncCmd,
	"config":   configCmd,
	"drand":    drandCmd,
	"dag":      dagCmd,
	"inspect":  inspectCmd,
	"leb128":   leb128Cmd,
	"log":      logCmd,
	"send":     msgSendCmd,
	"mpool":    mpoolCmd,
	"protocol": protocolCmd,
	"show":     showCmd,
	"swarm":    swarmCmd,
	"wallet":   walletCmd,
	"version":  versionCmd,
	"state":    stateCmd,
	"miner":    minerCmd,
	"paych":    paychCmd,
	"msig":     multisigCmd,
}

func init() {
	for k, v := range rootSubcmdsLocal {
		RootCmd.Subcommands[k] = v
	}

	for k, v := range rootSubcmdsDaemon {
		RootCmd.Subcommands[k] = v
		RootCmdDaemon.Subcommands[k] = v
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
	return node.NewClientEnv(ctx), nil
}

type executor struct {
	api   string
	token string
	exec  cmds.Executor
}

func (e *executor) Execute(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	if e.api == "" {
		return e.exec.Execute(req, re, env)
	}

	client := cmdhttp.NewClient(e.api, e.token, cmdhttp.ClientWithAPIPrefix(node.APIPrefix))

	return client.Execute(req, re, env)
}

func makeExecutor(req *cmds.Request, env interface{}) (cmds.Executor, error) {
	isDaemonRequired := requiresDaemon(req)
	var (
		apiInfo *APIInfo
		err     error
	)

	if isDaemonRequired {
		apiInfo, err = getAPIInfo(req)
		if err != nil {
			return nil, err
		}
	}
	if apiInfo == nil && isDaemonRequired {
		return nil, ErrMissingDaemon
	}
	if apiInfo == nil {
		apiInfo = &APIInfo{}
	}

	return &executor{
		api:   apiInfo.Addr,
		token: apiInfo.Token,
		exec:  cmds.NewExecutor(RootCmd),
	}, nil
}

type APIInfo struct {
	Addr  string
	Token string
}

func getAPIInfo(req *cmds.Request) (*APIInfo, error) {
	repoDir, _ := req.Options[OptionRepoDir].(string)
	repoDir, err := paths.GetRepoPath(repoDir)
	if err != nil {
		return nil, err
	}
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
		rpcAPI, err := repo.APIAddrFromRepoPath(repoDir)
		if err != nil {
			return nil, errors.Wrap(err, "can't find API endpoint address in environment, command-line, or local repo (is the daemon running?)")
		}
		rawAddr = rpcAPI //NOTICE command only use api
	}

	maddr, err := ma.NewMultiaddr(rawAddr)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to convert API endpoint address %s to a multiaddr", rawAddr))
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to dial API endpoint address %s", maddr))
	}

	token := ""
	if tk, ok := req.Options[OptionToken]; ok {
		tkArr := tk.([]string)
		if len(tkArr) > 0 {
			token = tkArr[0]
		}
	}
	if len(token) == 0 {
		tk, err := repo.APITokenFromRepoPath(repoDir)
		if err != nil {
			return nil, errors.Wrap(err, "can't find token in environment")
		}
		token = tk
	}

	return &APIInfo{
		Addr:  host,
		Token: token,
	}, nil
}

// nolint
func getAPIAddress(req *cmds.Request) (string, error) {
	var rawAddr string
	var err error
	// second highest precedence is env vars.
	if envapi := os.Getenv("VENUS_API"); envapi != "" {
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
		rpcAPI, err := repo.APIAddrFromRepoPath(repoDir)
		if err != nil {
			return "", errors.Wrap(err, "can't find API endpoint address in environment, command-line, or local repo (is the daemon running?)")
		}
		rawAddr = rpcAPI //NOTICE command only use api
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

func parseGasOptions(req *cmds.Request) (fbig.Int, fbig.Int, int64, error) {
	var (
		feecap      = types.ZeroFIL
		premium     = types.ZeroFIL
		ok          = false
		gasLimitInt = int64(0)
	)

	feecapOption := req.Options["gas-feecap"]
	if feecapOption != nil {
		feecap, ok = types.NewAttoFILFromString(feecapOption.(string), 10)
		if !ok {
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New("invalid gas price (specify FIL as a decimal number)")
		}
	}

	premiumOption := req.Options["gas-premium"]
	if premiumOption != nil {
		premium, ok = types.NewAttoFILFromString(premiumOption.(string), 10)
		if !ok {
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New("invalid gas price (specify FIL as a decimal number)")
		}
	}

	limitOption := req.Options["gas-limit"]
	if limitOption != nil {
		gasLimitInt, ok = limitOption.(int64)
		if !ok {
			msg := fmt.Sprintf("invalid gas limit: %s", limitOption)
			return types.ZeroFIL, types.ZeroFIL, 0, errors.New(msg)
		}
	}

	return feecap, premium, gasLimitInt, nil
}
