package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
	"github.com/filecoin-project/go-filecoin/util/version"
)

const USAGE = `Usage:  go-filecoin-migrate (describe|buildonly|migrate) [--verbose][--oldRepo][--newRepo]`

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
		mr := internal.NewMigrationRunner(getVerbose())
		if err := mr.Run(command); err != nil {
			exitErr(err.Error())
		}
	default:
		exitErr(fmt.Sprintf("Error: Invalid command: %s\n%s\n", command, USAGE))
	}
}

func exitErr(errstr string) {
	log.New(os.Stderr, "", 0).Println(errstr)
	os.Exit(1)
}

func showUsageAndExit(code int) {
	fmt.Println(USAGE)
	os.Exit(code)
}

func getCommand() string {
	return os.Args[1]
}

func getVerbose() bool {
	if _, found := findOpt("-v", os.Args); found {
		return true
	}
	_, res := findOpt("--verbose", os.Args)
	return res
}

func findOpt(str string, args []string) (string, bool) {
	for _, elem := range args {
		if strings.Contains(elem, str) {
			opt := strings.Split(elem, "=")
			if len(opt) > 1 {
				return opt[1], true
			}
			return "", true
		}
	}
	return "", false
}
