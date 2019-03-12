package commands

import (
	"fmt"
	"io"
	"time"

	cmds "gx/ipfs/QmQtQrtNioesAWtrx8csBvfY37gTe94d6wQ3VikZUjxD39/go-ipfs-cmds"
	peer "gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

// PingResult is a wrapper for the pongs returned from the ping channel. It
// contains 2 fields: Count and Time.
//
// * Count is a uint representing the index of pongs returned so far.
// * Time is the amount of time elapsed from ping to pong in seconds.
//
type PingResult struct {
	Count uint
	Time  time.Duration
}

var pingCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send echo request packets to p2p network members",
		ShortDescription: `
'ping' is a tool to test sending data to other nodes. It finds nodes
via the routing system, sends pings, waits for pongs, and prints out round-
trip latency information.
		`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer ID", true, false, "ID of peer to be pinged").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.UintOption("count", "c", "Number of ping messages to send").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peerID, err := peer.IDB58Decode(req.Arguments[0])
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
			result := &PingResult{
				Count: i,
				Time:  pong,
			}
			if err := re.Emit(result); err != nil {
				return err
			}
			if !pingChOpen {
				return errors.New("Ping channel closed by receiver")
			}
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, result *PingResult) error {
			milliseconds := result.Time.Seconds() * 1000
			fmt.Fprintf(w, "Pong received: seq=%d time=%.2f ms\n", result.Count, milliseconds) // nolint: errcheck
			return nil
		}),
	},
	Type: PingResult{},
}
