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
		option := args[0]
		if option == "--verbose" || option == "-v" {
			verbose = true
		} else {
			fmt.Println("\tInvalid option: ", option)
			os.Exit(1)
		}
	}

	switch command {
	case "-h", "--help":
		showUsage()
	case "describe", "buildonly", "migrate":
		cmd.Run(command, verbose)
	default:
		fmt.Println("\tInvalid command: ", command)
	}
}

func showUsage() {
	fmt.Println(`\tUsage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose]`)
}

func exitErr(err string) {
	fmt.Println(err)
	os.Exit(1)
}
func showUsageAndExit(code int) {
	showUsage()
	os.Exit(code)
}
