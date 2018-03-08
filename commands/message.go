package commands

import (
	"encoding/json"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var msgCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		// TODO: better description
		Tagline: "Manage messages",
	},
	Subcommands: map[string]*cmds.Command{
		"send": msgSendCmd,
		"wait": msgWaitCmd,
	},
}

var msgSendCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send a message", // This feels too generic...
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("target", true, false, "address to send message to"),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("value", "value to send with message"),
		cmdkit.StringOption("from", "address to send message from"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		target, err := types.ParseAddress(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		val, ok := req.Options["value"].(int)
		if !ok {
			val = 0
		}

		fromAddr, err := addressWithDefault(req.Options["from"], n)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		msg := types.NewMessage(fromAddr, target, big.NewInt(int64(val)), "", nil)

		if err := n.AddNewMessage(req.Context, msg); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		c, err := msg.Cid()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(c) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			return PrintString(w, c)
		}),
	},
}

type waitResult struct {
	Message   *types.Message
	Receipt   *types.MessageReceipt
	Signature *core.FunctionSignature
}

var msgWaitCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Wait for a message to appear in a mined block",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "the cid of the message to wait for"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("message", "print the whole message").WithDefault(true),
		cmdkit.BoolOption("receipt", "print the whole message receipt").WithDefault(true),
		cmdkit.BoolOption("return", "rint the return value from the receipt").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			re.SetError(errors.Wrap(err, "invalid message cid"), cmdkit.ErrNormal)
			return
		}

		waitForMessage(n, msgCid, func(blk *types.Block, msg *types.Message, receipt *types.MessageReceipt) {
			signature, err := n.GetSignature(req.Context, msg.To, msg.Method)
			if err != nil {
				re.SetError(errors.Wrap(err, "unable to determine return type"), cmdkit.ErrNormal)
				return
			}

			res := waitResult{
				Message:   msg,
				Receipt:   receipt,
				Signature: signature,
			}
			re.Emit(res) // nolint: errcheck
		})
	},
	Type: waitResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *waitResult) error {
			messageOpt, _ := req.Options["message"].(bool)
			receiptOpt, _ := req.Options["receipt"].(bool)
			returnOpt, _ := req.Options["return"].(bool)

			marshaled := []byte{}
			var err error
			if messageOpt {
				marshaled, err = appendJSON(res.Message, marshaled)
				if err != nil {
					return err
				}
			}

			if receiptOpt {
				marshaled, err = appendJSON(res.Receipt, marshaled)
				if err != nil {
					return err
				}
			}

			if returnOpt {
				val, err := abi.Deserialize(res.Receipt.Return, res.Signature.Return[0])
				if err != nil {
					return errors.Wrap(err, "unable to deserialize return value")
				}

				marshaled = append(marshaled, []byte(val.Val.(Stringer).String())...)
			}

			_, err = w.Write(marshaled)
			return err
		}),
	},
}

func appendJSON(val interface{}, out []byte) ([]byte, error) {
	m, err := json.MarshalIndent(val, "", "\t")
	if err != nil {
		return nil, err
	}
	out = append(out, m...)
	out = append(out, byte('\n'))
	return out, nil
}
