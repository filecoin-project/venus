package cmd

import (
	"context"
	rice "github.com/GeertJohan/go.rice"
	"github.com/docker/go-units"
	paramfetch "github.com/filecoin-project/go-paramfetch"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"
	"os"
	"os/signal"
	"syscall"
)

var fetchParamCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Fetch proving parameters",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("proving-params", true, false, "download params used creating proofs for given size, i.e. 32GiB"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		sectorSizeInt, err := units.RAMInBytes(req.Arguments[0])
		if err != nil {
			return err
		}
		sectorSize := uint64(sectorSizeInt)
		err = paramfetch.GetParams(ReqContext(req.Context), ParametersJSON(), sectorSize)
		if err != nil {
			return xerrors.Errorf("fetching proof parameters: %w", err)
		}
		return nil
	},
}

func ReqContext(cctx context.Context) context.Context {
	var (
		ctx  context.Context
		done context.CancelFunc
	)
	if cctx != nil {
		ctx = cctx
	} else {
		ctx = context.Background()
	}
	ctx, done = context.WithCancel(ctx)
	sigChan := make(chan os.Signal, 2)
	go func() {
		<-sigChan
		done()
	}()
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	return ctx
}

func ParametersJSON() []byte {
	return rice.MustFindBox("../fixtures/proof-params").MustBytes("parameters.json")
}
