package main

import (
	"context"
	"os"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-filecoin/cmd/go-filecoin"
)

func main() {

	// set default log level if no flags given
	var level logging.LogLevel
	var err error
	lvl := os.Getenv("GO_FILECOIN_LOG_LEVEL")
	if lvl == "" {
		level = logging.LevelInfo
	} else {
		level, err = logging.LevelFromString(lvl)
		if err != nil {
			level = logging.LevelInfo
		}
	}

	errCode := 1
	logging.SetAllLoggers(level)
	err = logging.SetLogLevel("dht", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("bitswap", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("graphsync", "info")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("heartbeat", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("blockservice", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("peerqueue", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("swarm", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("swarm2", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("basichost", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("dht_net", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("pubsub", "error")
	if err != nil {
		os.Exit(errCode)
	}
	err = logging.SetLogLevel("relay", "error")
	if err != nil {
		os.Exit(errCode)
	}

	code, _ := commands.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
