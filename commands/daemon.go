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

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZcLBXKaFe8ND5YHPkJRAwmhJGrVsi1JqDZNyJ4nRK5Mj/go-multiaddr-net"
	writer "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log/writer"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"
	cmdhttp "gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds/http"

	"github.com/filecoin-project/go-filecoin/api/impl"
	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
)

// exposed here, to be available during testing
var sigCh = make(chan os.Signal, 1)

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
		cmdkit.StringOption(BlockTime, "time a node waits before trying to mine the next block").WithDefault(mining.DefaultBlockTime.String()),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return daemonRun(req, re, env)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.Encoders[cmds.Text],
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	// third precedence is config file.
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	// second highest precedence is env vars.
	if envapi := os.Getenv("FIL_API"); envapi != "" {
		rep.Config().API.Address = envapi
	}

	// highest precedence is cmd line flag.
	if apiAddress, ok := req.Options[OptionAPI].(string); ok && apiAddress != "" {
		rep.Config().API.Address = apiAddress
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

	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	if fcn.OfflineMode {
		re.Emit("Filecoin node running in offline mode (libp2p is disabled)\n") // nolint: errcheck
	} else {
		re.Emit(fmt.Sprintf("My peer ID is %s\n", fcn.Host().ID().Pretty())) // nolint: errcheck
		for _, a := range fcn.Host().Addrs() {
			re.Emit(fmt.Sprintf("Swarm listening on: %s\n", a)) // nolint: errcheck
		}
	}

	if _, ok := req.Options[ELStdout].(bool); ok {
		writer.WriterGroup.AddWriter(os.Stdout)
	}

	return runAPIAndWait(req.Context, fcn, rep.Config(), req)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	return repo.OpenFSRepo(getRepoDir(req))
}

func runAPIAndWait(ctx context.Context, nd *node.Node, config *config.Config, req *cmds.Request) error {
	api := impl.New(nd)
	if err := nd.Start(ctx); err != nil {
		return err
	}
	defer nd.Stop(ctx)

	servenv := &Env{
		// TODO: should this be the passed in context?
		ctx:          context.Background(),
		api:          api,
		porcelainAPI: nd.PorcelainAPI,
		blockAPI:     nd.BlockAPI,
		retrievalAPI: nd.RetrievalAPI,
		storageAPI:   nd.StorageAPI,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(config.API.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(config.API.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(config.API.AccessControlAllowCredentials)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	maddr, err := ma.NewMultiaddr(config.API.Address)
	if err != nil {
		return err
	}

	// For the case when /ip4/127.0.0.1/tcp/0 is passed,
	// we want to fetch the new multiaddr from the listener, as it may (should)
	// have resolved to some other value. i.e. resolve port zero to real value.
	apiLis, err := manet.Listen(maddr)
	if err != nil {
		return err
	}
	config.API.Address = apiLis.Multiaddr().String()

	handler := http.NewServeMux()
	handler.Handle("/debug/pprof/", http.DefaultServeMux)
	handler.Handle(APIPrefix+"/", cmdhttp.NewHandler(servenv, rootCmdDaemon, cfg))

	apiserv := http.Server{
		Handler: handler,
	}

	go func() {
		err := apiserv.Serve(manet.NetListener(apiLis))
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// write our api address to file
	// TODO: use api.Repo() once implemented
	if err := nd.Repo.SetAPIAddr(config.API.Address); err != nil {
		return errors.Wrap(err, "Could not save API address to repo")
	}

	signal := <-sigCh
	fmt.Printf("Got %s, shutting down...\n", signal)

	// allow 5 seconds for clean shutdown. Ideally it would never take this long.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := apiserv.Shutdown(ctx); err != nil {
		fmt.Println("failed to shut down api server:", err)
	}

	return nil
}
