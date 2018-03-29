package commands

import (
	"context"
	"net"
	"os"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

const (
	// OptionAPI is the name of the option for specifying the api port.
	OptionAPI = "cmdapiaddr"
	// OptionRepoDir is the name of the option for specifying the directory of the repo.
	OptionRepoDir = "repodir"
	// APIPrefix is the prefix for the http version of the api.
	APIPrefix = "/api"
)

func defaultAPIAddr() string {
	// Until we have a config file, we need an easy way to influence the API
	// address for testing
	if envapi := os.Getenv("FIL_API"); envapi != "" {
		return envapi
	}

	return ":3453"
}

var rootCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "A decentralized storage network",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(OptionAPI, "set the api port to use").WithDefault(defaultAPIAddr()),
		cmdkit.StringOption(OptionRepoDir, "set the directory of the reop, defaults to ~/.filecoin"),
		cmds.OptionEncodingType,
		cmdkit.BoolOption("help", "Show the full command help text."),
		cmdkit.BoolOption("h", "Show a short version of the command help text."),
	},
	Subcommands: make(map[string]*cmds.Command),
}

// all top level commands. set during init() to avoid configuration loops.
var rootSubcmdsDaemon = map[string]*cmds.Command{
	"address":   addrsCmd,
	"bootstrap": bootstrapCmd,
	"chain":     chainCmd,
	"client":    clientCmd,
	"daemon":    daemonCmd,
	"dag":       dagCmd,
	"id":        idCmd,
	"init":      initCmd,
	"message":   msgCmd,
	"miner":     minerCmd,
	"mining":    miningCmd,
	"mpool":     mpoolCmd,
	"orderbook": orderbookCmd,
	"ping":      pingCmd,
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
	return &Env{ctx: ctx}, nil
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
	api := req.Options[OptionAPI].(string)
	isDaemonRunning, err := daemonRunning(api)
	if err != nil {
		return nil, err
	}

	if isDaemonRunning && req.Command == daemonCmd {
		return nil, ErrAlreadyRunning
	}

	if !isDaemonRunning && requiresDaemon(req) {
		return nil, ErrMissingDaemon
	}

	return &executor{
		api:     api,
		exec:    cmds.NewExecutor(rootCmd),
		running: isDaemonRunning,
	}, nil
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
