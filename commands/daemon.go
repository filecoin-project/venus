package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	libp2p "gx/ipfs/QmT68EgyFbnSs5rbHkNkFZQwjdHfqrJiGr3R6rwcwnzYLc/go-libp2p"
	cmds "gx/ipfs/QmWGgKRz5S24SqaAapF5PPCfYfLT7MexJZewN5M82CQTzs/go-ipfs-cmds"
	cmdhttp "gx/ipfs/QmWGgKRz5S24SqaAapF5PPCfYfLT7MexJZewN5M82CQTzs/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/node"
)

var daemonCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "start the filecoin daemon",
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("swarmlisten").WithDefault("/ip4/127.0.0.1/tcp/6000"),
	},
	Run: daemonRun,
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	api := req.Options[OptionAPI].(string)

	// TODO: this should be passed in from a config file, not an api flag
	libp2pOpts := node.Libp2pOptions(
		libp2p.ListenAddrStrings(req.Options["swarmlisten"].(string)),
	)

	node, err := node.New(req.Context, libp2pOpts)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	fmt.Println("My peer ID is", node.Host.ID().Pretty())
	for _, a := range node.Host.Addrs() {
		fmt.Println("Swarm listening on", a)
	}

	if err := runAPIAndWait(req.Context, node, api); err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
}

func runAPIAndWait(ctx context.Context, node *node.Node, api string) error {
	if err := node.Start(); err != nil {
		return err
	}

	servenv := &Env{
		ctx:  context.Background(),
		Node: node,
	}

	cfg := cmdhttp.NewServerConfig()
	cfg.APIPath = APIPrefix

	handler := cmdhttp.NewHandler(servenv, rootCmd, cfg)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	defer signal.Stop(sigc)

	apiserv := http.Server{
		Addr:    api,
		Handler: handler,
	}

	go func() {
		panic(http.ListenAndServe(api, handler))
	}()

	<-sigc
	fmt.Println("Got interrupt, shutting down...")

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	apiserv.Shutdown(ctx)
	node.Stop()

	return nil
}
