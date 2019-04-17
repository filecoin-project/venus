package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/filecoin-project/go-filecoin/tools/migration/cmd"
	"github.com/filecoin-project/go-filecoin/util/version"
)

func main() { // nolint: deadcode
	if len(os.Args) < 2 {
		showUsageAndExit(1)
	}

	if !version.Check(runtime.Version()) {
		exitErr(fmt.Sprintf("Invalid go version: %s", runtime.Version()))
	}

	command := getCommand()

	verbose, err := getVerbose()
	if err != nil {
		exitErr(err.Error())
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

// TODO: should this go to the same logs as the migration log? Maybe not.
func exitErr(errstr string) {
	log.New(os.Stderr, "", 0).Println(errstr)
	os.Exit(1)
}

func showUsageAndExit(code int) {
	fmt.Println(`    Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose]`)
	os.Exit(code)
}

func getCommand() string {
	return os.Args[1:][0]
}

func getVerbose() (bool, error) {
	if len(os.Args[1:]) > 1 {
		option := os.Args[2]
		if option == "--verbose" || option == "-v" {
			return true, nil
		} else {
			return false, errors.New("Invalid option")
		}
	}
	return false, nil
}
