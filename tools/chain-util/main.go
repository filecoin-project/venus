package main

import (
	"context"
	"fmt"
	"os"

	logging "github.com/ipfs/go-log"
	cli "gopkg.in/urfave/cli.v2"

	export "github.com/filecoin-project/go-filecoin/tools/chain-util/pkg/export"
)

var log = logging.Logger("chain-util")

func init() {
	logging.SetAllLoggers(logging.LevelWarn)
}

const (
	repoFlag = "repo"
	outFlag  = "out"
)

var exportCmd = &cli.Command{
	Name:  "export",
	Usage: "Export chain to car file",
	Flags: []cli.Flag{
		&cli.PathFlag{
			Name:  repoFlag,
			Usage: "the repo where go-filecoin was initialized",
		},
		&cli.PathFlag{
			Name:  outFlag,
			Usage: "the file to export the chain to",
		},
	},
	Action: func(cctx *cli.Context) error {
		cfg, err := parseFlags(cctx)
		if err != nil {
			return err
		}
		chainOut, err := export.NewChainExporter(cfg.repoPath, cfg.outFile)
		if err != nil {
			return err
		}
		if err := chainOut.Export(context.Background()); err == nil {
			fmt.Printf("Exported chain with head: %s to: %s", chainOut.Head, cfg.outFile.Name())
		}
		return err
	},
}

func main() {
	app := &cli.App{
		Name:     "chain-export",
		Commands: []*cli.Command{exportCmd},
	}
	app.Setup()

	if err := app.Run(os.Args); err != nil {
		log.Warn(err)
	}
	return

}
