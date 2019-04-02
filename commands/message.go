package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/types"
)

var msgCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send and monitor messages",
	},
	Subcommands: map[string]*cmds.Command{
		"send":   msgSendCmd,
		"status": msgStatusCmd,
		"wait":   msgWaitCmd,
	},
}

// MessageSendResult is the return type for message send command
type MessageSendResult struct {
	Cid     cid.Cid
	GasUsed types.GasUnits
	Preview bool
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
		cmdkit.IntOption("value", "Value to send with message in FIL"),
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
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				target,
				method,
			)
			if err != nil {
				return err
			}
			return re.Emit(&MessageSendResult{
				Cid:     cid.Cid{},
				GasUsed: usedGas,
				Preview: true,
			})
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

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: types.NewGasUnits(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MessageSendResult) error {
			if res.Preview {
				output := strconv.FormatUint(uint64(res.GasUsed), 10)
				_, err := w.Write([]byte(output))
				return err
			}
			return PrintString(w, res.Cid)
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
		cmdkit.StringArg("cid", true, false, "CID of the message to wait for"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("message", "Print the whole message").WithDefault(true),
		cmdkit.BoolOption("receipt", "Print the whole message receipt").WithDefault(true),
		cmdkit.BoolOption("return", "Print the return value from the receipt").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}

		fmt.Printf("waiting for: %s\n", req.Arguments[0])

		found := false
		err = GetPorcelainAPI(env).MessageWait(req.Context, msgCid, func(blk *types.Block, msg *types.SignedMessage, receipt *types.MessageReceipt) error {
			found = true
			sig, err2 := GetPorcelainAPI(env).ActorGetSignature(req.Context, msg.To, msg.Method)
			if err2 != nil && err2.Error() != "no method" && err2.Error() != "failed to load actor code" {
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

				marshaled = append(marshaled, []byte(val.Val.(fmt.Stringer).String())...)
			}

			_, err = w.Write(marshaled)
			return err
		}),
	},
}

// MessageStatusResult is the status of a message on chain or in the message queue/pool
type MessageStatusResult struct {
	InPool    bool // Whether the message is found in the mpool
	PoolMsg   *types.SignedMessage
	InOutbox  bool // Whether the message is found in the outbox
	OutboxMsg *core.QueuedMessage
	OnChain   bool // Whether the message is found on chain
	ChainMsg  *msg.ChainMessage
}

var msgStatusCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show status of a message",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of the message to inspect"),
	},
	Options: []cmdkit.Option{},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}

		api := GetPorcelainAPI(env)
		result := MessageStatusResult{}

		// Look in message pool
		result.PoolMsg, result.InPool = api.MessagePoolGet(msgCid)

		// Look in outbox
		for _, addr := range api.OutboxQueues() {
			for _, qm := range api.OutboxQueueLs(addr) {
				cid, err := qm.Msg.Cid()
				if err != nil {
					return err
				}
				if cid.Equals(msgCid) {
					result.InOutbox = true
					result.OutboxMsg = qm
				}
			}
		}

		// Look on chain
		result.ChainMsg, result.OnChain, err = api.MessageFind(req.Context, msgCid)
		if err != nil {
			return err
		}
		return re.Emit(&result)
	},
	Type: &MessageStatusResult{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *MessageStatusResult) error {
			sw := NewSilentWriter(w)
			var msg *types.SignedMessage
			if res.InOutbox {
				msg = res.OutboxMsg.Msg
				sw.Printf("In outbox: %s, sent at height %d\n", res.OutboxMsg.Msg.From, res.OutboxMsg.Stamp)
			}
			if res.InPool {
				msg = res.PoolMsg
				sw.Printf("In mpool\n")
			}
			if res.OnChain {
				msg = res.ChainMsg.Message
				sw.Printf("On chain at height %d, receipt %v\n", res.ChainMsg.Block.Height, res.ChainMsg.Receipt)
			}
			if msg != nil {
				sw.Println(msg.String())
			}
			return sw.Error()
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
