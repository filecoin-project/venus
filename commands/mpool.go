package commands

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
	"gx/ipfs/Qmf46mr235gtyxizkKUkTH5fo62Thza2zwXR4DWC7rkoqF/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/types"
)

var mpoolCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage the message pool",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":   mpoolLsCmd,
		"show": mpoolShowCmd,
		"rm":   mpoolRemoveCmd,
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

		pending, err := GetPorcelainAPI(env).MessagePoolWait(req.Context, messageCount)
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
				_ = PrintString(w, c)
			}
			return nil
		}),
	},
}

var mpoolShowCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show content of an outstanding message",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "The CID of the message to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid message cid")
		}

		msg, ok := GetPorcelainAPI(env).MessagePoolGet(msgCid)
		if !ok {
			return fmt.Errorf("message %s not found in pool (already mined?)", msgCid)
		}
		return re.Emit(msg)
	},
	Type: &types.SignedMessage{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, msg *types.SignedMessage) error {
			_, err := fmt.Fprintf(w, `Message Details
To:        %s
From:      %s
Nonce:     %s
Value:     %s
Method:    %s
Params:    %s
Gas price: %s
Gas limit: %s
Signature: %s
`,
				msg.To,
				msg.From,
				strconv.FormatUint(uint64(msg.Nonce), 10),
				msg.Value,
				msg.Method,
				base64.StdEncoding.EncodeToString(msg.Params),
				msg.GasPrice.String(),
				strconv.FormatUint(uint64(msg.GasLimit), 10),
				base64.StdEncoding.EncodeToString(msg.Signature),
			)
			return err
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
