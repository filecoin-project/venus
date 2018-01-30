package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/cmd"
)

func main() {
	code, err := cmd.Run(os.Args, os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
