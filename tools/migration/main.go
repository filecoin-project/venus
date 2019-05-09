package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-filecoin/tools/migration/internal"
)

const defaultLogFilePath = "~/.filecoin-migration-logs"

// USAGE is the usage documentation for the migration tool
const USAGE = `
USAGE
	go-filecoin-migrate -h|--help
	go-filecoin-migrate (describe|buildonly|migrate) --old-repo=<repolink> [-h|--help] [-v|--verbose]
	go-filecoin-migrate install --old-repo=<repolink> --new-repo=<migrated-repo> [-v|--verbose]

COMMANDS
	describe
		prints a description of what the current migration will do
	buildonly
		runs the migration and validations, but does not install the newly migrated repo
		at the --old-repo symlink
	migrate
		runs migration, validations, and installs newly migrated repo at --old-repo symlink
	install
		installs a newly migrated repo

REQUIRED ARGUMENTS
	--old-repo
		The symlink location of this node's filecoin home directory. This is required even for the
		'describe' command, as its repo version helps determine which migration to run. This
		must be a symbolic link or migration will not proceed.

	--new-repo
		the location of a newly migrated repo. This is required only for the install command and
		otherwise ignored.

OPTIONS
	-h, --help
		This message
	-v --verbose
		Print diagnostic messages to stdout
        --log-file
                The path of the file for writing detailed log output

EXAMPLES
	for a migration from version 1 to 2:
	go-filecoin-migrate migrate --old-repo=~/.filecoin
		Migrates then installs the repo. Migrated repo will be in ~/.filecoin_1_2_<timestamp>
		and symlinked to ~/.filecoin

	go-filecoin-migrate migrate --old-repo=/opt/filecoin
		Migrates then installs the repo. Migrated repo will be in /opt/filecoin_1_2_<timestamp> 
		and symlinked to /opt/filecoin

	go-filecoin-migrate build-only --old-repo=/opt/filecoin 
		Runs migration steps only. Migrated repo will be in /opt/filecoin_1_2_<timestamp>
		and symlinked to /opt/filecoin

	go-filecoin-migrate install --old-repo=/opt/filecoin --new-repo=/opt/filecoin-123445566860 --verbose
		swaps out the link at /opt/filecoin to point to /opt/filecoin-123445566860, as long as
		/opt/filecoin is a symlink and /opt/filecoin-123445566860 has an up-to-date version.
`

func main() { // nolint: deadcode
	if len(os.Args) < 2 {
		showUsageAndExit(1)
	}

	command := os.Args[1]

	switch command {
	case "-h", "--help":
		showUsageAndExit(0)
	case "describe", "buildonly", "migrate", "install":
		logFile, err := openLogFile()
		if err != nil {
			exitErr(err.Error())
		}

		logger := internal.NewLogger(logFile, getVerbose())

		oldRepoOpt, found := findOpt("old-repo", os.Args)
		if !found {
			exitErr(fmt.Sprintf("--old-repo is required\n%s\n", USAGE))
		}

		var newRepoOpt string
		if command == "install" {
			newRepoOpt, found = findOpt("new-repo", os.Args)
			if !found {
				exitErr(fmt.Sprintf("--new-repo is required for 'install'\n%s\n", USAGE))
			}
		}

		runner, err := internal.NewMigrationRunner(logger, command, oldRepoOpt, newRepoOpt)
		if err != nil {
			exitErr(err.Error())
		}
		runResult := runner.Run()
		if runResult.Err != nil {
			exitErr(runResult.Err.Error())
		}
		if runResult.NewRepoPath != "" {
			logger.Print(fmt.Sprintf("New repo location: %s", runResult.NewRepoPath))
		}
		if runResult.NewVersion != runResult.OldVersion {
			logger.Print(fmt.Sprintf("Repo has been migrated to version %d", runResult.NewVersion))
		}
	default:
		exitErr(fmt.Sprintf("invalid command: %s\n%s\n", command, USAGE))
	}
}

// exitError exit(1)s the executable with the given error String
func exitErr(errstr string) {
	log.New(os.Stderr, "", 0).Println("Error: " + errstr)
	os.Exit(1)
}

// showUsageAndExit prints out USAGE and exits with the given code.
func showUsageAndExit(code int) {
	fmt.Print(USAGE)
	os.Exit(code)
}

// getVerbose parses os.Args looking for -v or --verbose.
// returns whether it was found.
func getVerbose() bool {
	if _, found := findOpt("-v", os.Args); found {
		return true
	}
	_, res := findOpt("--verbose", os.Args)
	return res
}

// openLogFile opens the log file from getLogFilePath
func openLogFile() (*os.File, error) {
	path, err := getLogFilePath()
	if err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE, 0644)
}

// getLogFilePath returns the path of the logfile.
func getLogFilePath() (string, error) {
	if logPath, found := findOpt("--log-file", os.Args); found {
		return logPath, nil
	}

	return homedir.Expand(defaultLogFilePath)
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
