package main

import (
	"os"
	"strconv"

	oldlogging "gx/ipfs/QmcaSwFc5RBg8yCq54QURwEU4nwjfCpjbpmaAm4VbdGLKv/go-logging"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	// TODO: make configurable - this should be done via a command like go-ipfs
	// something like:
	//		`go-filecoin log level "system" "level"`
	// TODO: find a better home for this
	// TODO fix this in go-log 4 == INFO
	n, err := strconv.Atoi(os.Getenv("GO_FILECOIN_LOG_LEVEL"))
	if err != nil {
		n = 3
	}

	logging.SetAllLoggers(oldlogging.Level(n))

	// TODO implement help text like so:
	// https://github.com/ipfs/go-ipfs/blob/master/core/commands/root.go#L91
	// TODO don't panic if run without a command.
	code, _ := commands.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
