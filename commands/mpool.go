package commands

import (
	"fmt"
	"io"

	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View the mempool of outstanding messages",
	},
	Options: []cmdkit.Option{
		cmdkit.UintOption("wait-for-count", "Block until this number of messages are in the pool").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		messageCount, _ := req.Options["wait-for-count"].(uint)

		pending, err := GetAPI(env).Mpool().View(req.Context, messageCount)
		if err != nil {
			return err
		}

		return re.Emit(pending)
	},
	Type: []*types.SignedMessage{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, msgs *[]*types.SignedMessage) error {
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
