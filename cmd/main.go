package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-cmds/cli"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/repo"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"

	OptionToken = "token"
	// OptionRepoDir is the name of the option for specifying the directory of the repo.
	OptionRepoDir = "repo"

	OptionLegacyRepoDir = "repodir"

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

	ULimit = "manage-fdlimit"

	// AutoSealIntervalSeconds configures the daemon to check for and seal any staged sectors on an interval.
	//AutoSealIntervalSeconds = "auto-seal-interval-seconds"

	// SwarmAddress is the multiaddr for this Filecoin node
	SwarmAddress = "swarmlisten"

	// SwarmPublicRelayAddress is a public address that the venus node
	// will listen on if it is operating as a relay.  We use this to specify
	// the public ip:port of a relay node that is sitting behind a static
	// NAT mapping.
	SwarmPublicRelayAddress = "swarmrelaypublic"

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

	AuthServiceURL = "auth-url"
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
  config <key> [<value>] - Get and set filecoin config values
  daemon                 - Start a long-running daemon process
  wallet                 - Manage your filecoin wallets
  msig                   - Interact with a multisig wallet

VIEW DATA STRUCTURES
  chain                  - Inspect the filecoin blockchain
  sync                   - Inspect the filecoin Sync
  dag                    - Interact with IPLD DAG objects
  show                   - Get human-readable representations of filecoin objects

NETWORK COMMANDS
  swarm                  - Interact with the swarm
  drand                  - retrieve drand randomness

MESSAGE COMMANDS
  send                   - Send message
  mpool                  - Manage the message pool

State COMMANDS
  wait-msg               - Wait for a message to appear on chain
  search-msg             - Search to see whether a message has appeared on chain
  power                  - Query network or miner power
  sectors                - Query the sector set of a miner
  active-sectors         - Query the active sector set of a miner
  sector                 - Get miner sector info
  get-actor              - Print actor information
  lookup                 - Find corresponding ID address
  sector-size            - Look up miners sector size
  get-deal               - View on-chain deal info
  miner-info             - Retrieve miner information
  network-version        - MReturns the network version
  list-actor             - list all actors

Paych COMMANDS 
  paych                  - Manage payment channels

Cid COMMANDS
  manifest-cid-from-car  - Get the manifest CID from a car file

TOOL COMMANDS
  inspect                - Show info about the venus node
  leb128                 - Leb128 cli encode/decode
  log                    - Interact with the daemon event log output
  protocol               - Show protocol parameter details
  version                - Show venus version information
  seed                   - Seal sectors for genesis miner
  fetch                  - Fetch proving parameters
`,
	},
	Options: []cmds.Option{
		cmds.StringsOption(OptionToken, "set the auth token to use"),
		cmds.StringOption(OptionAPI, "set the api port to use"),
		cmds.StringOption(OptionRepoDir, OptionLegacyRepoDir, "set the repo directory, defaults to ~/.venus"),
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
	"cid":     cidCmd,
}

// all top level commands, available on daemon. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"chain":    chainCmd,
	"sync":     syncCmd,
	"drand":    drandCmd,
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

	client := cmdhttp.NewClient(e.api, cmdhttp.ClientWithAPIPrefix(node.APIPrefix), cmdhttp.ClientWithHeader("Authorization", "Bearer "+e.token))

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

	rawAddr = strings.Trim(rawAddr, " \n\t")
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

func requiresDaemon(req *cmds.Request) bool {
	for cmd := range rootSubcmdsLocal {
		if len(req.Path) > 0 && req.Path[0] == cmd {
			return false
		}
	}
	return true
}
