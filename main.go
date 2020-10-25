package main

import (
	"context"
	"os"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

func main() {

	// set default log level if no flags given

	logging.SetAllLoggers(logging.LevelFatal)

	code, _ := commands.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
