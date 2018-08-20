package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	manet "gx/ipfs/QmV6FjemM1K8oXjrvuq3wuVWWoU2TLDPmNnKrxHzY3v6Ai/go-multiaddr-net"
	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds/http"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	ma "gx/ipfs/QmYmsdtJ3HsodkePE3eU3TsCaP2YvPZJ4LoXnNkDE5Tpt7/go-multiaddr"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/api/impl"
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
		cmdkit.BoolOption(MockMineMode),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := daemonRun(req, re, env); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.Encoders[cmds.Text],
	},
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	if apiAddress, ok := req.Options[OptionAPI].(string); ok && apiAddress != "" {
		rep.Config().API.Address = apiAddress
	}

	if swarmAddress, ok := req.Options[SwarmListen].(string); ok && swarmAddress != "" {
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

	if mockMineMode, ok := req.Options[MockMineMode].(bool); ok {
		opts = append(opts, func(c *node.Config) error {
			c.MockMineMode = mockMineMode
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

	return runAPIAndWait(req.Context, fcn, rep.Config(), req)
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	return repo.OpenFSRepo(getRepoDir(req))
}

func runAPIAndWait(ctx context.Context, node *node.Node, config *config.Config, req *cmds.Request) error {
	api := impl.New(node)

	if err := api.Daemon().Start(ctx); err != nil {
		return err
	}

	servenv := &Env{
		// TODO: should this be the passed in context?
		ctx: context.Background(),
		api: api,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix
	cfg.SetAllowedOrigins(config.API.AccessControlAllowOrigin...)
	cfg.SetAllowedMethods(config.API.AccessControlAllowMethods...)
	cfg.SetAllowCredentials(config.API.AccessControlAllowCredentials)

	handler := cmdhttp.NewHandler(servenv, rootCmd, cfg)

	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	maddr, err := ma.NewMultiaddr(config.API.Address)
	if err != nil {
		return err
	}

	_, host, err := manet.DialArgs(maddr)
	if err != nil {
		return err
	}

	apiserv := http.Server{
		Addr:    host,
		Handler: handler,
	}

	go func() {
		err := apiserv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// write our api address to file
	// TODO: use api.Repo() once implemented
	if err := node.Repo.SetAPIAddr(config.API.Address); err != nil {
		return errors.Wrap(err, "Could not save API address to repo")
	}

	<-sigCh
	fmt.Println("Got interrupt, shutting down...")

	// allow 5 seconds for clean shutdown. Ideally it would never take this long.
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	if err := apiserv.Shutdown(ctx); err != nil {
		fmt.Println("failed to shut down api server:", err)
	}

	return api.Daemon().Stop(ctx)
}
