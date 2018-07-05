package commands

import (
	"context"
	"fmt"
	"io"

	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View the mempool",
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("wait-for-count", "block until this number of messages are in the pool").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		messageCount, _ := req.Options["wait-for-count"].(int)
		ctx := context.Background()

		n := GetNode(env)
		pending := n.MsgPool.Pending()
		if len(pending) < messageCount {
			subscription, err := n.PubSub.Subscribe(node.MessageTopic)
			if err != nil {
				return err
			}

			for len(pending) < messageCount {
				_, err = subscription.Next(ctx)
				if err != nil {
					return err
				}
				pending = n.MsgPool.Pending()
			}
		}
		re.Emit(pending) // nolint: errcheck

		return nil
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
