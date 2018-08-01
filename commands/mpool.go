package commands

import (
	"context"
	"fmt"
	"io"

	"gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core/node"
	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View the mempool",
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("wait-for-count", "block until this number of messages are in the pool").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		messageCount, _ := req.Options["wait-for-count"].(int)
		ctx := context.Background()

		n := GetNode(env)
		pending := n.MsgPool.Pending()
		if len(pending) < messageCount {
			subscription, err := n.PubSub.Subscribe(node.MessageTopic)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			for len(pending) < messageCount {
				_, err = subscription.Next(ctx)
				if err != nil {
					re.SetError(err, cmdkit.ErrNormal)
					return
				}
				pending = n.MsgPool.Pending()
			}
		}
		re.Emit(pending) // nolint: errcheck
	},
	Type: []*types.Message{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, msgs *[]*types.Message) error {
			for _, msg := range *msgs {
				c, err := msg.Cid()
				if err != nil {
					return err
				}
				fmt.Fprintln(w, c.String()) // nolint: errcheck
			}
			return nil
		}),
	},
}
