package main

import (
	"context"
	"os"

	logging "github.com/ipfs/go-log/v2"

	commands "github.com/filecoin-project/venus/cmd/go-filecoin"
)

func main() {

	logging.SetAllLoggers(logging.LevelFatal)

	//// set default log level if no flags given
	//var level logging.LogLevel
	//var err error
	//lvl := os.Getenv("GO_FILECOIN_LOG_LEVEL")
	//if lvl == "" {
	//	level = logging.LevelInfo
	//} else {
	//	level, err = logging.LevelFromString(lvl)
	//	if err != nil {
	//		level = logging.LevelInfo
	//	}
	//}
	//
	//logging.SetAllLoggers(level)
	//logging.SetLogLevel("dht", "error")          // nolint: errcheck
	//logging.SetLogLevel("bitswap", "error")      // nolint: errcheck
	//logging.SetLogLevel("graphsync", "info")     // nolint: errcheck
	//logging.SetLogLevel("heartbeat", "error")    // nolint: errcheck
	//logging.SetLogLevel("blockservice", "error") // nolint: errcheck
	//logging.SetLogLevel("peerqueue", "error")    // nolint: errcheck
	//logging.SetLogLevel("swarm", "error")        // nolint: errcheck
	//logging.SetLogLevel("swarm2", "error")       // nolint: errcheck
	//logging.SetLogLevel("basichost", "error")    // nolint: errcheck
	//logging.SetLogLevel("dht_net", "error")      // nolint: errcheck
	//logging.SetLogLevel("pubsub", "error")       // nolint: errcheck
	//logging.SetLogLevel("relay", "error")        // nolint: errcheck

	code, _ := commands.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
