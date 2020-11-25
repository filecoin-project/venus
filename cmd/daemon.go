package cmd

import (
	"fmt"
	_ "net/http/pprof" // nolint: golint
	"os"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/paths"
	"github.com/filecoin-project/venus/pkg/journal"
	"github.com/filecoin-project/venus/pkg/repo"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var daemonCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Start a long-running daemon process",
	},
	Options: []cmds.Option{
		cmds.StringOption(SwarmAddress, "multiaddress to listen on for filecoin network connections"),
		cmds.StringOption(SwarmPublicRelayAddress, "public multiaddress for routing circuit relay traffic.  Necessary for relay nodes to provide this if they are not publically dialable"),
		cmds.BoolOption(OfflineMode, "start the node without networking"),
		cmds.BoolOption(ELStdout),
		cmds.BoolOption(IsRelay, "advertise and allow filecoin network traffic to be relayed through this node"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return daemonRun(req, re)
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter) error {
	// third precedence is config file.
	rep, err := getRepo(req)
	if err != nil {
		return err
	}
	config := rep.Config()

	// second highest precedence is env vars.
	if envAPI := os.Getenv("FIL_API"); envAPI != "" {
		config.API.RustFulAddress = envAPI
	}

	// highest precedence is cmd line flag.
	if flagAPI, ok := req.Options[OptionAPI].(string); ok && flagAPI != "" {
		config.API.RustFulAddress = flagAPI
	}

	if swarmAddress, ok := req.Options[SwarmAddress].(string); ok && swarmAddress != "" {
		config.Swarm.Address = swarmAddress
	}

	if publicRelayAddress, ok := req.Options[SwarmPublicRelayAddress].(string); ok && publicRelayAddress != "" {
		config.Swarm.PublicRelayAddress = publicRelayAddress
	}

	opts, err := node.OptionsFromRepo(rep)
	if err != nil {
		return err
	}

	if offlineMode, ok := req.Options[OfflineMode].(bool); ok {
		opts = append(opts, node.OfflineMode(offlineMode))
	}

	if isRelay, ok := req.Options[IsRelay].(bool); ok && isRelay {
		opts = append(opts, node.IsRelay())
	}

	journal, err := journal.NewZapJournal(rep.JournalPath())
	if err != nil {
		return err
	}
	opts = append(opts, node.JournalConfigOption(journal))

	// Monkey-patch network parameters option will set package variables during node build
	opts = append(opts, node.MonkeyPatchNetworkParamsOption(config.NetworkParams))

	// Instantiate the node.
	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	if fcn.OfflineMode {
		_ = re.Emit("Filecoin node running in offline mode (libp2p is disabled)\n")
	} else {
		_ = re.Emit(fmt.Sprintf("My peer ID is %s\n", fcn.Host().ID().Pretty()))
		for _, a := range fcn.Host().Addrs() {
			_ = re.Emit(fmt.Sprintf("Swarm listening on: %s\n", a))
		}
	}

	if _, ok := req.Options[ELStdout].(bool); ok {
		_ = re.Emit("--" + ELStdout + " option is deprecated\n")
	}

	// Start the node.
	if err := fcn.Start(req.Context); err != nil {
		return err
	}
	defer fcn.Stop(req.Context)

	// Run API server around the node.
	ready := make(chan interface{}, 1)
	go func() {
		<-ready
		lines := []string{
			fmt.Sprintf("Rust API server listening on %s\n", config.API.RustFulAddress),
			fmt.Sprintf("JsonRpc API server listening on %s\n", config.API.JSONRPCAddress),
		}
		_ = re.Emit(lines)
	}()

	// The request is expected to remain open so the daemon uses the request context.
	// Pass a new context here if the flow changes such that the command should exit while leaving
	// a forked deamon running.
	return fcn.RunRPCAndWait(req.Context, RootCmdDaemon, ready)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	repoDir, _ := req.Options[OptionRepoDir].(string)
	repoDir, err := paths.GetRepoPath(repoDir)
	if err != nil {
		return nil, err
	}
	return repo.OpenFSRepo(repoDir, repo.Version)
}
