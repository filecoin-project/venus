package commands

import (
	"fmt"
	"time"

	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-core/peer"

	//"github.com/libp2p/go-libp2p/p2p/protocol/ping"
	"github.com/pkg/errors"
)

// PingResult is a wrapper for the pongs returned from the ping channel. It
// contains 2 fields: Count and Time.
//
// * Count is a uint representing the index of pongs returned so far.
// * Time is the amount of time elapsed from ping to pong in seconds.
//
type PingResult struct {
	Count uint
	RTT   time.Duration
}

var pingCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send echo request packets to p2p network members",
		ShortDescription: `
'ping' is a tool to test sending data to other nodes. It finds nodes
via the routing system, sends pings, waits for pongs, and prints out round-
trip latency information.
		`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer ID", true, false, "ID of peer to be pinged").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.UintOption("count", "c", "Number of ping messages to send").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peerID, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer address '%s': %s", req.Arguments[0], err)
		}

		numPings, _ := req.Options["count"].(uint)

		pingCh, err := GetPorcelainAPI(env).NetworkPing(req.Context, peerID)
		if err != nil {
			return err
		}

		for i := uint(0); numPings == 0 || i < numPings; i++ {
			pong, pingChOpen := <-pingCh
			if pong.Error != nil {
				return pong.Error
			}
			result := &PingResult{
				Count: i,
				RTT:   pong.RTT,
			}
			if err := re.Emit(result); err != nil {
				return err
			}
			if !pingChOpen {
				return errors.New("Ping channel closed by sender")
			}
		}

		return nil
	},
	Type: PingResult{},
}
