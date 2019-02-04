package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	cmdkit "gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/mthdsig"
	"github.com/filecoin-project/go-filecoin/porcelain"
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
		cmdkit.StringArg("target", true, false, "Address of the actor to send the message to"),
		cmdkit.StringArg("method", false, false, "The method to invoke on the target actor"),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("value", "Value to send with message, in AttoFIL"),
		cmdkit.StringOption("from", "Address to send message from"),
		priceOption,
		limitOption,
		previewOption,
		// TODO: (per dignifiedquire) add an option to set the nonce and method explicitly
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		target, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		val, ok := req.Options["value"].(int)
		if !ok {
			val = 0
		}

		o := req.Options["from"]
		var fromAddr address.Address
		if o != nil {
			var err error
			fromAddr, err = address.NewFromString(o.(string))
			if err != nil {
				return errors.Wrap(err, "invalid from address")
			}
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		method, ok := req.Options["method"].(string)
		if !ok {
			method = ""
		}

		if preview {
			usedGas, err := porcelain.MessagePreviewWithDefaultAddress(
				req.Context,
				GetPlumbingAPI(env),
				fromAddr,
				target,
				method,
			)
			if err != nil {
				return err
			}
			return re.Emit(strconv.FormatUint(uint64(usedGas), 10))
		}

		c, err := GetPorcelainAPI(env).MessageSendWithDefaultAddress(
			req.Context,
			fromAddr,
			target,
			types.NewAttoFILFromFIL(uint64(val)),
			gasPrice,
			gasLimit,
			method,
		)
		if err != nil {
			return err
		}

		return re.Emit(c.String())
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res string) error {
			_, err := w.Write([]byte(res))
			return err
		}),
	},
}

// WaitResult is the result of a message wait call.
type WaitResult struct {
	Message   *types.SignedMessage
	Receipt   *types.MessageReceipt
	Signature *exec.FunctionSignature
}

var msgWaitCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Wait for a message to appear in a mined block",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "The cid of the message to wait for"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("message", "Print the whole message").WithDefault(true),
		cmdkit.BoolOption("receipt", "Print the whole message receipt").WithDefault(true),
		cmdkit.BoolOption("return", "Print the return value from the receipt").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid message cid")
		}

		fmt.Printf("waiting for: %s\n", req.Arguments[0])

		found := false
		err = GetPorcelainAPI(env).MessageWait(req.Context, msgCid, func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt) error {
			found = true
			sig, err2 := GetPorcelainAPI(env).ActorGetSignature(req.Context, msg.To, msg.Method)
			if err2 != nil && err2 != mthdsig.ErrNoMethod && err2 != mthdsig.ErrNoActorImpl {
				return errors.Wrap(err2, "Couldn't get signature for message")
			}

			res := WaitResult{
				Message: msg,
				Receipt: receipt,
				// Signature is required to decode the output.
				Signature: sig,
			}
			re.Emit(&res) // nolint: errcheck

			return nil
		})

		if err != nil && !found {
			return err
		}
		return nil
	},
	Type: WaitResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *WaitResult) error {
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
