package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	// TODO don't panic if run without a command.
	code, err := commands.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
