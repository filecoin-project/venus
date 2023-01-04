package main

import (
	"fmt"
	"log"
	"os"

	"github.com/filecoin-project/venus/venus-devtool/api-gen/common"
	"github.com/filecoin-project/venus/venus-devtool/api-gen/proxy"
	"github.com/filecoin-project/venus/venus-devtool/util"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "api-gen",
		Usage:                "generate api related codes for venus-shared",
		EnableBashCompletion: true,
		Flags:                []cli.Flag{},
		Commands: []*cli.Command{
			proxyCmd,
			clientCmd,
			docGenCmd,
			mockCmd,
		},
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}

var proxyCmd = &cli.Command{
	Name:  "proxy",
	Flags: []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		if err := util.LoadExtraInterfaceMeta(); err != nil {
			return err
		}
		for _, target := range common.ApiTargets {
			err := proxy.GenProxyForAPI(target)
			if err != nil {
				log.Fatalf("got error while generating proxy codes for %s: %s", target.Type, err)
			}
		}
		return nil
	},
}
