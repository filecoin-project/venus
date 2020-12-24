package main

import (
	"context"
	"github.com/filecoin-project/venus/cmd"
	logging "github.com/ipfs/go-log"
	"os"
)

func main() {
	// set default log level if no flags given
	lvl := os.Getenv("GO_FILECOIN_LOG_LEVEL")
	if lvl == "" {
		logging.SetAllLoggers(logging.LevelInfo)
		logging.SetLogLevel("dht", "error")          // nolint: errcheck
		logging.SetLogLevel("bitswap", "error")      // nolint: errcheck
		logging.SetLogLevel("graphsync", "info")     // nolint: errcheck
		logging.SetLogLevel("heartbeat", "error")    // nolint: errcheck
		logging.SetLogLevel("blockservice", "error") // nolint: errcheck
		logging.SetLogLevel("peerqueue", "error")    // nolint: errcheck
		logging.SetLogLevel("swarm", "info")         // nolint: errcheck
		logging.SetLogLevel("swarm2", "info")        // nolint: errcheck
		logging.SetLogLevel("basichost", "error")    // nolint: errcheck
		logging.SetLogLevel("dht_net", "error")      // nolint: errcheck
		logging.SetLogLevel("pubsub", "error")       // nolint: errcheck
		logging.SetLogLevel("relay", "error")        // nolint: errcheck
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
