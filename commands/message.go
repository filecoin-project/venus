package commands

import (
	"encoding/json"
	"fmt"
	"io"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/node"
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
		// TODO: (per dignifiedquire) add an option to set the nonce and method explicitly
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		target, err := types.NewAddressFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		val, ok := req.Options["value"].(int)
		if !ok {
			val = 0
		}

		fromAddr, err := fromAddress(req.Options, n)
		if err != nil {
			return err
		}

		msg, err := node.NewMessageWithNextNonce(req.Context, n, fromAddr, target, types.NewAttoFILFromFIL(uint64(val)), "", nil)
		if err != nil {
			return err
		}

		if err := n.AddNewMessage(req.Context, msg); err != nil {
			return err
		}

		c, err := msg.Cid()
		if err != nil {
			return err
		}

		re.Emit(c) // nolint: errcheck

		return nil
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
	Signature *exec.FunctionSignature
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
		cmdkit.BoolOption("return", "print the return value from the receipt").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		fmt.Println("waiting for", req.Arguments[0])
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid message cid")
		}

		var found bool
		err = n.ChainMgr.WaitForMessage(req.Context, msgCid, func(blk *types.Block, msg *types.Message,
			receipt *types.MessageReceipt) error {
			signature, err := n.GetSignature(req.Context, msg.To, msg.Method)
			if err != nil && err != node.ErrNoMethod {
				return errors.Wrap(err, "unable to determine return type")
			}

			res := waitResult{
				Message:   msg,
				Receipt:   receipt,
				Signature: signature,
			}
			re.Emit(res) // nolint: errcheck
			found = true

			return nil
		})
		if err != nil && !found {
			return err
		}
		return nil
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

			if returnOpt && res.Receipt != nil && res.Signature != nil {
				val, err := abi.Deserialize(res.Receipt.Return[0], res.Signature.Return[0])
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
