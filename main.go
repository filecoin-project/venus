package main

import (
	"fmt"
	"os"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	// TODO: make configurable
	// TODO: find a better home for this
	logging.Configure(logging.LevelInfo)

	// TODO implement help text like so:
	// https://github.com/ipfs/go-ipfs/blob/master/core/commands/root.go#L91
	// TODO don't panic if run without a command.
	code, err := commands.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
