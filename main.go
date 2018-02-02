package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	// TODO implement help text like so:
	// https://github.com/ipfs/go-ipfs/blob/master/core/commands/root.go#L91
	// TODO don't panic if run without a command.
	code, err := commands.Run(os.Args, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
