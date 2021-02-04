package main

import (
	"context"
	_ "net/http/pprof"
	"os"

	"github.com/filecoin-project/venus/cmd"
	logging "github.com/ipfs/go-log"
)

func main() {
	// set default log level if no flags given
	lvl := os.Getenv("GO_FILECOIN_LOG_LEVEL")
	if lvl == "" {
		logging.SetAllLoggers(logging.LevelInfo)
		logging.SetLogLevel("beacon", "error")               // nolint: errcheck
		logging.SetLogLevel("peer-tracker", "error")         // nolint: errcheck
		logging.SetLogLevel("dht", "error")                  // nolint: errcheck
		logging.SetLogLevel("bitswap", "error")              // nolint: errcheck
		logging.SetLogLevel("graphsync", "info")             // nolint: errcheck
		logging.SetLogLevel("heartbeat", "error")            // nolint: errcheck
		logging.SetLogLevel("blockservice", "error")         // nolint: errcheck
		logging.SetLogLevel("peerqueue", "error")            // nolint: errcheck
		logging.SetLogLevel("swarm", "error")                // nolint: errcheck
		logging.SetLogLevel("swarm2", "error")               // nolint: errcheck
		logging.SetLogLevel("basichost", "error")            // nolint: errcheck
		logging.SetLogLevel("dht_net", "error")              // nolint: errcheck
		logging.SetLogLevel("pubsub", "error")               // nolint: errcheck
		logging.SetLogLevel("relay", "error")                // nolint: errcheck
		logging.SetLogLevel("dht/RtRefreshManager", "error") // nolint: errcheck

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
