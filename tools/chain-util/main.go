package main

import (
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log"
	cli "gopkg.in/urfave/cli.v2"

	export "github.com/filecoin-project/go-filecoin/tools/chain-util/pkg/export"
)

var log = logging.Logger("chain-util")

func init() {
	logging.SetAllLoggers(logging.LevelDebug)
}

const (
	repoFlag = "repo"
	headFlag = "head"
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
		&cli.StringSliceFlag{
			Name:  headFlag,
			Usage: "the CID to be used as the root of the export",
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
		chainOut, err := export.NewChainExporter(filepath.Join(cfg.repoPath, "badger/"), cfg.outFile)
		if err != nil {
			return err
		}
		return chainOut.Export(cfg.headKey)
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
