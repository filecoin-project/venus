package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/filecoin-project/go-filecoin/tools/migration/cmd"
	"github.com/filecoin-project/go-filecoin/util/version"
)

func main() {
	if len(os.Args) < 2 {
		showUsageAndExit(1)
	}

	if !version.Check(runtime.Version()) {
		exitErr(fmt.Sprintf("Invalid go version: %s", runtime.Version()))
	}

	args := os.Args[1:]
	command := args[0]
	verbose := false
	if len(args) > 1 {
		option := args[1]
		if option == "--verbose" || option == "-v" {
			verbose = true
		} else {
			fmt.Println("\tInvalid option: ", option)
			os.Exit(1)
		}
	}

	switch command {
	case "-h", "--help":
		showUsageAndExit(0)
	case "describe", "buildonly", "migrate", "install":
		if err := cmd.Run(command, verbose); err != nil {
			exitErr(err.Error())
		}
	default:
		exitErr(fmt.Sprintf("Invalid command: %s", command))
	}
}

func exitErr(errstr string) {
	fmt.Printf("Error: %s\n", errstr)
	os.Exit(1)
}
func showUsageAndExit(code int) {
	fmt.Println(`    Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose]`)
	os.Exit(code)
}
