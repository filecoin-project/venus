package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/config"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/repo"
)

// exposed here, to be available during testing
var sigCh = make(chan os.Signal, 1)

var daemonCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Start a long-running daemon-process",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(SwarmListen),
		cmdkit.BoolOption(OfflineMode),
	},
	Run: daemonRun,
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.Encoders[cmds.Text],
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	if apiAddress, ok := req.Options[OptionAPI].(string); ok {
		rep.Config().API.Address = apiAddress
	}

	if swarmAddress, ok := req.Options[SwarmListen].(string); ok {
		rep.Config().Swarm.Address = swarmAddress
	}

	opts, err := node.OptionsFromRepo(rep)
	if err != nil {
		return err
	}

	if offlineMode, ok := req.Options[OfflineMode].(bool); ok {
		opts = append(opts, func(c *node.Config) error {
			c.OfflineMode = offlineMode
			return nil
		})
	}

	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	if fcn.OfflineMode {
		re.Emit("Filecoin node running in offline mode (libp2p is disabled)\n") // nolint: errcheck
	} else {
		re.Emit(fmt.Sprintf("My peer ID is %s\n", fcn.Host.ID().Pretty())) // nolint: errcheck
		for _, a := range fcn.Host.Addrs() {
			re.Emit(fmt.Sprintf("Swarm listening on: %s\n", a)) // nolint: errcheck
		}
	}

	return runAPIAndWait(req.Context, fcn, rep.Config())
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	return repo.OpenFSRepo(getRepoDir(req))
}

func runAPIAndWait(ctx context.Context, node *node.Node, config *config.Config) error {
	if err := node.Start(); err != nil {
		return err
	}

	servenv := &Env{
		ctx:  context.Background(),
		node: node,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(config.API.AccessControlAllowOrigin...)

	handler := cmdhttp.NewHandler(servenv, rootCmd, cfg)

	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	apiserv := http.Server{
		Addr:    config.API.Address,
		Handler: handler,
	}

	go func() {
		err := apiserv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-sigCh
	fmt.Println("Got interrupt, shutting down...")

	// allow 5 seconds for clean shutdown. Ideally it would never take this long.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := apiserv.Shutdown(ctx); err != nil {
		fmt.Println("failed to shut down api server:", err)
	}
	node.Stop()

	return nil
}
