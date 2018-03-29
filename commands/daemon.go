package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds/http"
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
		cmdkit.StringOption("swarmlisten"),
	},
	Run: daemonRun,
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	rep, err := getRepo(req)
	if err != nil {
		return err
	}

	if apiAddress, ok := req.Options[OptionAPI].(string); ok {
		rep.Config().API.Address = apiAddress
	}

	opts := node.OptionsFromRepo(rep)

	if swarmAddress, ok := req.Options["swarmlisten"].(string); ok {
		opts = append(opts,
			// TODO: this should be passed in from a config file, not an api flag
			node.Libp2pOptions(libp2p.ListenAddrStrings(swarmAddress)),
		)
	}

	// TODO: since init and startup are currently one, do this here...
	if err := node.Init(req.Context, rep); err != nil {
		return err
	}

	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		return err
	}

	re.Emit(fmt.Sprintf("My peer ID is %s", fcn.Host.ID().Pretty())) // nolint: errcheck
	for _, a := range fcn.Host.Addrs() {
		re.Emit(fmt.Sprintf("Swarm listening on: %s", a)) // nolint: errcheck
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
