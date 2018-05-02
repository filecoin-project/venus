package main

import (
	"os"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	// TODO: make configurable - this should be done via a command like go-ipfs
	// something like:
	//		`go-filecoin log level "system" "level"`
	// TODO: find a better home for this
	// TODO fix this in go-log 4 == INFO
	logging.SetAllLoggers(4)

	// TODO implement help text like so:
	// https://github.com/ipfs/go-ipfs/blob/master/core/commands/root.go#L91
	// TODO don't panic if run without a command.
	code, _ := commands.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
