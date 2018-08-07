package commands

import (
	"context"
	"fmt"
	"io"
	"time"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmY51bqSM5XgxQZqsBrQcRkKTnCb8EKpJpR9K6Qax7Njco/go-libp2p/p2p/protocol/ping"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	peer "gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"
)

type pingResult struct {
	Time    time.Duration
	Text    string
	Success bool
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
		cmdkit.StringArg("peer ID", true, true, "ID of peer to be pinged.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("count", "n", "Number of ping messages to send.").WithDefault(10),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {

		n := GetNode(env)

		peerID, err := peer.IDB58Decode(req.Arguments[0])

		if err != nil {
			err = fmt.Errorf("failed to parse peer address '%s': %s", req.Arguments[0], err)
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if peerID == n.Host.ID() {
			re.SetError(ErrCannotPingSelf, cmdkit.ErrNormal)
			return
		}

		numPings, _ := req.Options["count"].(int)
		if numPings <= 0 {
			err := fmt.Errorf("error: ping count must be greater than 0, was %d", numPings)
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		err = pingPeer(req.Context, n.Ping, peerID, numPings, time.Second, re)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, p *pingResult) error {
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
	Type: pingResult{},
}

const pingTimeout = time.Second * 10

// TODO: this sort of logic should be a helper function in the pingservice package.
func pingPeer(ctx context.Context, p *ping.PingService, pid peer.ID, count int, delay time.Duration, re cmds.ResponseEmitter) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	times, err := p.Ping(ctx, pid)
	if err != nil {
		return err
	}

	re.Emit(&pingResult{Text: fmt.Sprintf("PING %s", pid)}) // nolint: errcheck

	for i := 0; i < count; i++ {
		select {
		case dur := <-times:
			re.Emit(&pingResult{Time: dur, Success: true}) // nolint: errcheck
		case <-time.After(pingTimeout):
			re.Emit(&pingResult{Text: "error: timeout"}) // nolint: errcheck
		case <-ctx.Done():
			return nil
		}

		time.Sleep(delay)
	}
	return nil
}
