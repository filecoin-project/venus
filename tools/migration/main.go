package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

const USAGE = `
USAGE
	go-filecoin-migrate (describe|buildonly|migrate) --old-repo=<repodir> [--new-repo=<newrepo-prefix] [-h|--help] [-v|--verbose]

COMMANDS
	describe
		prints a description of what the current migration will do
	buildonly
		runs the migration, but does not install the newly migrated repo
	migrate
		runs the migration, runs validation tests on the migrated repo, then
		installs the newly migrated repo

REQUIRED ARGUMENTS
	--old-repo
		The location of this node's filecoin home directory. This is required even for the
		'describe' command, as the its repo version helps determine which migration to run.

OPTIONS
	-h, --help
		This message
	-v --verbose
		Print diagnostic messages to stdout
	--new-repo
		The prefix for the migrated repo. A directory prefixed with this 
		path will be created to hold the copy of the old repo for migration, named 
		with a timestamp and migration versions. 

		Provide this if you want the migrated repo to be in a different directory, on a 
        different device, or you just prefer a different naming scheme.

		Ensure the parent directory exists; go-filecoin-migrate will not create tree
        structure.

EXAMPLES
	for a migration from version 1 to 2:
	go-filecoin-migrate migrate --old-repo=~/.filecoin
		Migrates then installs the repo. Migrated repo will be in ~/.filecoin_1_2_<timestamp>

	go-filecoin-migrate migrate --old-repo=/opt/filecoin
		Migrates then installs the repo. Migrated repo will be in /opt/filecoin_1_2_<timestamp> 

	go-filecoin-migrate build-only --old-repo=/opt/filecoin --new-repo=/tmp/somedir
		Runs migration steps only. Migrated repo will be in /tmp/somedir_1_2_<timestamp>
`

func main() { // nolint: deadcode
	if len(os.Args) < 2 {
		showUsageAndExit(1)
	}

	command := getCommand()
	switch command {
	case "-h", "--help":
		showUsageAndExit(0)
	case "describe", "buildonly", "migrate", "install":
		oldRepoOpt, found := findOpt("old-repo", os.Args)

		if found == false {
			exitErr(fmt.Sprintf("Error: --old-repo is required\n%s\n", USAGE))
		}

		newRepoPrefixOpt, _ := findOpt("new-repo", os.Args)
		runner := internal.NewMigrationRunner(getVerbose(), command, oldRepoOpt, newRepoPrefixOpt)
		if err := runner.Run(); err != nil {
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

// findOpt fetches option values.
// returns:  string: value of option set with "=". If not set, returns ""
//           bool:  true if option was found, false if not
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
