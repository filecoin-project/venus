package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "apis",
		Usage:                "devtool for api compatible checks between lotus & venus",
		EnableBashCompletion: true,
		Flags:                []cli.Flag{},
		Commands: []*cli.Command{
			checksumCmd,
		},
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}
