package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {
	code, err := commands.Run(os.Args, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
