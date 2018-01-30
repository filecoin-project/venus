package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	cmds "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds"
	cmdhttp "gx/ipfs/Qmc5paX4ECBARnAKkcAmUYHBGor228Tkfxeya3Nu2KRL46/go-ipfs-cmds/http"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/node"
)

var daemonCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "start the filecoin daemon",
	},
	Run: daemonRun,
}

func daemonRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
	api := req.Options[OptionAPI].(string)

	node := node.New()
	if err := startNode(node, api); err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}
}

func startNode(node *node.Node, api string) error {
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

	go func() {
		panic(http.ListenAndServe(api, handler))
	}()

	<-sigc
	fmt.Println("Got interrupt, shutting down...")
	go node.Stop()

	return nil
}
