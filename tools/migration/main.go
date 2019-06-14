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
	describe	prints a description of what the current migration will do

	buildonly	runs the migration and validations, but does not install the newly migrated 
				repo at the --old-repo symlink

	migrate		runs migration, validations, and installs newly migrated repo at 
				--old-repo symlink

	install		installs a newly migrated repo

REQUIRED ARGUMENTS
	--old-repo	the symlink location of this node's filecoin home directory. This is required
		even for the 'describe' command, as its repo version helps determine which migration 
		to run. This must be a symbolic link or migration will not proceed.

	--new-repo	the location of a newly migrated repo. This is required only for the 
		install command and otherwise ignored.

OPTIONS
	-h, --help     This message
	-v --verbose   Print diagnostic messages to stdout
	--log-file     The path of the file for writing detailed log output

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
	case "describe":
		logger, err := newLoggerWithVerbose(true)
		if err != nil {
			exitErr(err.Error())
		}

		oldRepoOpt := findOldRepoOrExit(logger)

		// Errors are handled inside runRunner
		_ = runRunner(logger, command, oldRepoOpt, "")

	case "buildonly", "migrate", "install":
		logger, err := newLoggerWithVerbose(getVerbose())
		if err != nil {
			exitErr(err.Error())
		}

		oldRepoOpt := findOldRepoOrExit(logger)

		var newRepoOpt string
		var found bool

		if command == "install" {
			newRepoOpt, found = findOpt("new-repo", os.Args)
			if !found {
				exitErrCloseLogger(fmt.Sprintf("--new-repo is required for 'install'\n%s\n", USAGE), logger)
			}
		}

		runResult := runRunner(logger, command, oldRepoOpt, newRepoOpt)
		if runResult.NewRepoPath != "" {
			logger.Printf("New repo location: %s", runResult.NewRepoPath)
		}
		if runResult.NewVersion != runResult.OldVersion {
			logger.Printf("Repo has been migrated to version %d", runResult.NewVersion)
		}
		err = logger.Close()
		if err != nil {
			exitErr(err.Error())
		}
	default:
		exitErr(fmt.Sprintf("invalid command: %s\n%s\n", command, USAGE))
	}
}

func runRunner(logger *internal.Logger, command string, oldRepoOpt string, newRepoOpt string) internal.RunResult {
	runner, err := internal.NewMigrationRunner(logger, command, oldRepoOpt, newRepoOpt)
	if err != nil {
		exitErrCloseLogger(err.Error(), logger)
	}
	runResult := runner.Run()
	if runResult.Err != nil {
		exitErrCloseLogger(runResult.Err.Error(), logger)
	}
	return runResult
}

func findOldRepoOrExit(logger *internal.Logger) string {
	oldRepoOpt, found := findOpt("old-repo", os.Args)
	if !found {
		exitErrCloseLogger(fmt.Sprintf("--old-repo is required\n%s\n", USAGE), logger)
	}
	return oldRepoOpt
}

// exitError exit(1)s the executable with the given error String
func exitErr(errstr string) {
	log.New(os.Stderr, "", 0).Println("Error: " + errstr)
	os.Exit(1)
}

// exitErrorCloseLogger closes the logger and calls exitError
func exitErrCloseLogger(errstr string, logger *internal.Logger) {
	err := logger.Close()
	if err != nil {
		errstr = fmt.Sprintf("%s. Error closing logfile when reporting this error %s", errstr, err.Error())
	}
	exitErr(errstr)
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

// newLoggerWithVerbose opens a new logger & logfile with verboseness set to `verb`
func newLoggerWithVerbose(verb bool) (*internal.Logger, error) {
	path, err := getLogFilePath()
	if err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return internal.NewLogger(logFile, verb), nil
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
