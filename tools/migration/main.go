package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
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

	switch command {
	case "-h", "--help":
		showUsageAndExit(0)
	case "describe", "buildonly", "migrate", "install":
		mr := internal.MigrationRunner{}
		if err := mr.Run(command, getVerbose()); err != nil {
			exitErr(err.Error())
		}
	default:
		exitErr(fmt.Sprintf("Error: Invalid command: %s", command))
	}
}

// TODO: should this go to the same logs as the migration log? Maybe not.
func exitErr(errstr string) {
	log.New(os.Stderr, "", 0).Println(errstr)
	os.Exit(1)
}

func showUsageAndExit(code int) {
	fmt.Println(`    Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose][--oldRepo][--newRepo]`)
	os.Exit(code)
}

func getCommand() string {
	return os.Args[1:][0]
}

func getVerbose() bool {
	if _, found := findString("-v", os.Args); found {
		return true
	}
	_, res := findString("--verbose", os.Args)
	return res
}

func findString(str string, args []string) (string, bool) {
	for _, elem := range args {
		if elem == str {
			return elem, true
		}
	}
	return "", false
}
