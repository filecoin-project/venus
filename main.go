package main

import (
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/cmd"
)

var (
	logLevel string
)

func init() {
	flag.StringVar(&logLevel, "log-level", "", "Set log level (default: info)")
	// read logger configuration file if present
if _, err := os.Stat("logger.json"); !os.IsNotExist(err) {
    file, err := ioutil.ReadFile("logger.json")
    if err != nil {
        log.Fatalf("Error reading logger configuration file: %v", err)
    }
    if err := logging.SetLogLevelConfigString(string(file)); err != nil {
        log.Fatalf("Error setting logger configuration: %v", err)
    }
}
}

func main() {
	flag.Parse()

	// set default log level if no flags given
	if logLevel == "" {
		logging.SetAllLoggers(logging.LevelInfo)
	} else {
		level, err := logging.LevelFromString(logLevel)
		if err != nil {
			log.Fatalf("Error setting log level: %v", err)
		}
		logging.SetAllLoggers(level)
	}

	// read logger configuration file if present
	if _, err := os.Stat("logger.json"); !os.IsNotExist(err) {
		file, err := ioutil.ReadFile("logger.json")
		if err != nil {
			log.Fatalf("Error reading logger configuration file: %v", err)
		}
		if err := logging.SetLogLevelConfigString(string(file)); err != nil {
			log.Fatalf("Error setting logger configuration: %v", err)
		}
	}

	if len(os.Args) > 1 && os.Args[1] == "-v" {
		os.Args[1] = "version"
	}

	code, err := cmd.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		log.Printf("Error running command: %v", err)
		os.Exit(1)
	}

	os.Exit(code)
}
