package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	"gx/ipfs/QmNh1kGFFdsPu79KNSaL4NUKUPb4Eiz4KHdMtFY6664RDp/go-libp2p/p2p/protocol/ping"
	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
)

type pingResult struct {
	Time    time.Duration
	Text    string
	Success bool
}

var pingCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send echo request packets to libp2p hosts.",
		ShortDescription: `
'ping' is a tool to test sending data to other nodes. It finds nodes
via the routing system, sends pings, waits for pongs, and prints out round-
trip latency information.
		`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peer ID", true, true, "ID of peer to be pinged.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("count", "n", "Number of ping messages to send.").WithDefault(10),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		peerID, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			re.SetError(fmt.Errorf("failed to parse peer address '%s': %s", req.Arguments[0], err), cmdkit.ErrNormal)
			return
		}

		if peerID == n.Host.ID() {
			re.SetError("cannot ping self", cmdkit.ErrNormal)
			return
		}

		numPings, _ := req.Options["count"].(int)
		if numPings <= 0 {
			re.SetError(fmt.Errorf("error: ping count must be greater than 0, was %d", numPings), cmdkit.ErrNormal)
		}

		pingPeer(req.Context, n.Ping, peerID, numPings, time.Second, re)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, p *pingResult) error {
			if len(p.Text) > 0 {
				fmt.Fprintln(w, p.Text)
			} else if p.Success {
				fmt.Fprintf(w, "Pong received: time=%.2f ms\n", p.Time.Seconds()*1000)
			} else {
				fmt.Fprintf(w, "Pong failed\n")
			}
			return nil
		}),
	},
	Type: pingResult{},
}

const pingTimeout = time.Second * 10

// TODO: this sort of logic should be a helper function in the pingservice package.
func pingPeer(ctx context.Context, p *ping.PingService, pid peer.ID, count int, delay time.Duration, re cmds.ResponseEmitter) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	times, err := p.Ping(ctx, pid)
	if err != nil {
		re.SetError(err, cmdkit.ErrNormal)
		return
	}

	re.Emit(&pingResult{Text: fmt.Sprintf("PING %s", pid)})

	for i := 0; i < count; i++ {
		select {
		case dur := <-times:
			re.Emit(&pingResult{Time: dur, Success: true})
		case <-time.After(pingTimeout):
			re.Emit(&pingResult{Text: "error: timeout"})
		case <-ctx.Done():
			return
		}

		time.Sleep(delay)
	}
}
