package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	libp2p "gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p"
	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

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
		cmdkit.StringOption("swarmlisten").WithDefault("/ip4/127.0.0.1/tcp/6000"),
	},
	Run: daemonRun,
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	api := req.Options[OptionAPI].(string)

	repo, err := getRepo(req)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	opts := node.OptionsFromRepo(repo)

	opts = append(opts,
		// TODO: this should be passed in from a config file, not an api flag
		node.Libp2pOptions(libp2p.ListenAddrStrings(req.Options["swarmlisten"].(string))),
	)

	fcn, err := node.New(req.Context, opts...)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	re.Emit(fmt.Sprintf("My peer ID is %s", fcn.Host.ID().Pretty())) // nolint: errcheck
	for _, a := range fcn.Host.Addrs() {
		re.Emit(fmt.Sprintf("Swarm listening on: %s", a)) // nolint: errcheck
	}

	if err := runAPIAndWait(req.Context, fcn, api); err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
}

func getRepo(req *cmds.Request) (repo.Repo, error) {
	// TODO: takes the request to make the repo loading configurable.
	return repo.NewInMemoryRepo(), nil
}

func runAPIAndWait(ctx context.Context, node *node.Node, api string) error {
	if err := node.Start(); err != nil {
		return err
	}

	servenv := &Env{
		ctx:  context.Background(),
		node: node,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix

	handler := cmdhttp.NewHandler(servenv, rootCmd, cfg)

	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	apiserv := http.Server{
		Addr:    api,
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
