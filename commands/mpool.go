package commands

import (
	"fmt"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage the message pool",
	},
	Subcommands: map[string]*cmds.Command{
		"ls": mpoolLsCmd,
		"rm": mpoolRemoveCmd,
	},
}

var mpoolLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "View the pool of outstanding messages",
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

var mpoolRemoveCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Delete a message from the message pool",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "The CID of the message to delete"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid message cid")
		}

		GetPorcelainAPI(env).MessagePoolRemove(msgCid)

		return nil
	},
}
