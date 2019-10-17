package main

import (
	"context"
	"os"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/commands"
)

func main() {

	// set default log level if no flags given
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("dht", "error")          // nolint: errcheck
	logging.SetLogLevel("bitswap", "error")      // nolint: errcheck
	logging.SetLogLevel("graphsync", "info")     // nolint: errcheck
	logging.SetLogLevel("heartbeat", "error")    // nolint: errcheck
	logging.SetLogLevel("blockservice", "error") // nolint: errcheck
	logging.SetLogLevel("peerqueue", "error")    // nolint: errcheck
	logging.SetLogLevel("swarm", "error")        // nolint: errcheck
	logging.SetLogLevel("swarm2", "error")       // nolint: errcheck
	logging.SetLogLevel("basichost", "error")    // nolint: errcheck
	logging.SetLogLevel("dht_net", "error")      // nolint: errcheck
	logging.SetLogLevel("pubsub", "error")       // nolint: errcheck
	logging.SetLogLevel("relay", "error")        // nolint: errcheck

	// TODO implement help text like so:
	// https://github.com/ipfs/go-ipfs/blob/master/core/commands/root.go#L91
	// TODO don't panic if run without a command.
	code, _ := commands.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
