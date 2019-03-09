package commands

import (
	"fmt"
	"io"
	"time"

	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/api"
)

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
		cmdkit.StringArg("peer ID", true, false, "ID of peer to be pinged.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.UintOption("count", "c", "Number of ping messages to send.").WithDefault(10),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peerID, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer address '%s': %s", req.Arguments[0], err)
		}

		numPings, _ := req.Options["count"].(uint)

		ch, err := GetAPI(env).Ping().Ping(req.Context, peerID, numPings, time.Second)
		if err != nil {
			return err
		}

		for p := range ch {
			if err := re.Emit(p); err != nil {
				return err
			}
		}
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, p *api.PingResult) error {
			if len(p.Text) > 0 {
				fmt.Fprintln(w, p.Text) // nolint: errcheck
			} else if p.Success {
				fmt.Fprintf(w, "Pong received: time=%.2f ms\n", p.Time.Seconds()*1000) // nolint: errcheck
			} else {
				fmt.Fprintf(w, "Pong failed\n") // nolint: errcheck
			}
			return nil
		}),
	},
	Type: api.PingResult{},
}
