package commands

import (
	"fmt"
	"io"

	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View the mempool",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)
		re.Emit(n.MsgPool.Pending()) // nolint: errcheck

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
				fmt.Fprintln(w, c.String())
			}
			return nil
		}),
	},
}
