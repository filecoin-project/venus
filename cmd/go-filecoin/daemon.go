package commands

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // nolint: golint
	"os"
	"os/signal"
	"syscall"
	"time"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdhttp "github.com/ipfs/go-ipfs-cmds/http"
	writer "github.com/ipfs/go-log/writer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
	"github.com/filecoin-project/go-filecoin/internal/pkg/config"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/journal"
	"github.com/filecoin-project/go-filecoin/internal/pkg/repo"
)

var daemonCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Start a long-running daemon process",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(SwarmAddress, "multiaddress to listen on for filecoin network connections"),
		cmdkit.StringOption(SwarmPublicRelayAddress, "public multiaddress for routing circuit relay traffic.  Necessary for relay nodes to provide this if they are not publically dialable"),
		cmdkit.BoolOption(OfflineMode, "start the node without networking"),
		cmdkit.BoolOption(ELStdout),
		cmdkit.BoolOption(IsRelay, "advertise and allow filecoin network traffic to be relayed through this node"),
		cmdkit.StringOption(BlockTime, "time a node waits before trying to mine the next block").WithDefault(consensus.DefaultBlockTime.String()),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return daemonRun(req, re)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.Encoders[cmds.Text],
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter) error {
	// third precedence is config file.
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	// second highest precedence is env vars.
	if envAPI := os.Getenv("FIL_API"); envAPI != "" {
		rep.Config().API.Address = envAPI
	}

	// highest precedence is cmd line flag.
	if flagAPI, ok := req.Options[OptionAPI].(string); ok && flagAPI != "" {
		rep.Config().API.Address = flagAPI
	}

	if swarmAddress, ok := req.Options[SwarmAddress].(string); ok && swarmAddress != "" {
		rep.Config().Swarm.Address = swarmAddress
	}

	if publicRelayAddress, ok := req.Options[SwarmPublicRelayAddress].(string); ok && publicRelayAddress != "" {
		rep.Config().Swarm.PublicRelayAddress = publicRelayAddress
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

	durStr, ok := req.Options[BlockTime].(string)
	if !ok {
		return errors.New("Bad block time passed")
	}

	blockTime, err := time.ParseDuration(durStr)
	if err != nil {
		return errors.Wrap(err, "Bad block time passed")
	}
	opts = append(opts, node.BlockTime(blockTime))
	opts = append(opts, node.ClockConfigOption(clock.NewSystemClock()))

	journal, err := journal.NewZapJournal(rep.JournalPath())
	if err != nil {
		return err
	}
	opts = append(opts, node.JournalConfigOption(journal))

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
		writer.WriterGroup.AddWriter(os.Stdout)
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
		_ = re.Emit(fmt.Sprintf("API server listening on %s\n", rep.Config().API.Address))
	}()

	var terminate = make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(terminate)

	// The request is expected to remain open so the daemon uses the request context.
	// Pass a new context here if the flow changes such that the command should exit while leaving
	// a forked deamon running.
	return RunAPIAndWait(req.Context, fcn, rep.Config().API, ready, terminate)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	repoDir, _ := req.Options[OptionRepoDir].(string)
	repoDir, err := paths.GetRepoPath(repoDir)
	if err != nil {
		return nil, err
	}
	return repo.OpenFSRepo(repoDir, repo.Version)
}

// RunAPIAndWait starts an API server and waits for it to finish.
// The `ready` channel is closed when the server is running and its API address has been
// saved to the node's repo.
// A message sent to or closure of the `terminate` channel signals the server to stop.
func RunAPIAndWait(ctx context.Context, nd *node.Node, config *config.APIConfig, ready chan interface{}, terminate chan os.Signal) error {
	servenv := &Env{
		blockMiningAPI: nd.BlockMining.BlockMiningAPI,
		ctx:            ctx,
		inspectorAPI:   NewInspectorAPI(nd.Repo),
		porcelainAPI:   nd.PorcelainAPI,
		retrievalAPI:   nd.RetrievalProtocol.RetrievalAPI,
		storageAPI:     nd.StorageProtocol.StorageAPI,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(config.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(config.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(config.AccessControlAllowCredentials)

	maddr, err := ma.NewMultiaddr(config.Address)
	if err != nil {
		return err
	}

	// Listen on the configured address in order to bind the port number in case it has
	// been configured as zero (i.e. OS-provided)
	apiListener, err := manet.Listen(maddr)
	if err != nil {
		return err
	}

	handler := http.NewServeMux()
	handler.Handle("/debug/pprof/", http.DefaultServeMux)
	handler.Handle(APIPrefix+"/", cmdhttp.NewHandler(servenv, rootCmdDaemon, cfg))

	apiserv := http.Server{
		Handler: handler,
	}

	go func() {
		err := apiserv.Serve(manet.NetListener(apiListener))
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Write the resolved API address to the repo
	config.Address = apiListener.Multiaddr().String()
	if err := nd.Repo.SetAPIAddr(config.Address); err != nil {
		return errors.Wrap(err, "Could not save API address to repo")
	}
	// Signal that the sever has started and then wait for a signal to stop.
	close(ready)
	received := <-terminate
	if received != nil {
		fmt.Println("Received signal", received)
	}
	fmt.Println("Shutting down...")

	// Allow a grace period for clean shutdown.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := apiserv.Shutdown(ctx); err != nil {
		fmt.Println("Error shutting down API server:", err)
	}

	return nil
}
