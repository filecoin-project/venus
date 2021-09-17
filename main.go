package main

import (
	"context"
	_ "net/http/pprof"
	"os"

	"github.com/filecoin-project/venus/cmd"
	logging "github.com/ipfs/go-log/v2"
)

func main() {
	// set default log level if no flags given
	lvl := os.Getenv("GO_FILECOIN_LOG_LEVEL")
	if lvl == "" {
		logging.SetAllLoggers(logging.LevelInfo)
		_ = logging.SetLogLevel("beacon", "error")
		_ = logging.SetLogLevel("peer-tracker", "error")
		_ = logging.SetLogLevel("dht", "error")
		_ = logging.SetLogLevel("bitswap", "error")
		_ = logging.SetLogLevel("graphsync", "info")
		_ = logging.SetLogLevel("heartbeat", "error")
		_ = logging.SetLogLevel("dagservice", "error")
		_ = logging.SetLogLevel("peerqueue", "error")
		_ = logging.SetLogLevel("swarm", "error")
		_ = logging.SetLogLevel("swarm2", "error")
		_ = logging.SetLogLevel("basichost", "error")
		_ = logging.SetLogLevel("dht_net", "error")
		_ = logging.SetLogLevel("pubsub", "error")
		_ = logging.SetLogLevel("relay", "error")
		_ = logging.SetLogLevel("dht/RtRefreshManager", "error")
	} else {
		level, err := logging.LevelFromString(lvl)
		if err != nil {
			level = logging.LevelInfo
		}
		logging.SetAllLoggers(level)
	}

	code, _ := cmd.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
