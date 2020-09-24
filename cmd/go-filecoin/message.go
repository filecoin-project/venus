package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/util/adt"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

var msgCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send and monitor messages",
	},
	Subcommands: map[string]*cmds.Command{
		"send":       msgSendCmd,
		"sendsigned": signedMsgSendCmd,
		"status":     msgStatusCmd,
		"wait":       msgWaitCmd,
	},
}

// MessageSendResult is the return type for message send command
type MessageSendResult struct {
	Cid     cid.Cid
	GasUsed gas.Unit
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
		cmdkit.StringOption("value", "Value to send with message in FIL"),
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

		rawVal := req.Options["value"]
		if rawVal == nil {
			rawVal = "0"
		}
		val, ok := types.NewAttoFILFromFILString(rawVal.(string))
		if !ok {
			return errors.New("mal-formed value")
		}

		fromAddr, err := fromAddrOrDefault(req, env)
		if err != nil {
			return err
		}

		gasPrice, gasLimit, preview, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		methodID := builtin.MethodSend
		methodInput, ok := req.Options["method"].(uint64)
		if ok {
			methodID = abi.MethodNum(methodInput)
		}

		if preview {
			usedGas, err := GetPorcelainAPI(env).MessagePreview(
				req.Context,
				fromAddr,
				target,
				methodID,
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

		c, _, err := GetPorcelainAPI(env).MessageSend(
			req.Context,
			fromAddr,
			target,
			val,
			gasPrice,
			gasLimit,
			methodID,
			[]byte{},
		)
		if err != nil {
			return err
		}

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: gas.NewGas(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
}

var signedMsgSendCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send a signed message",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("message", true, false, "Signed Json message"),
	},
	Options: []cmdkit.Option{},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msg := req.Arguments[0]

		m := types.SignedMessage{}

		bmsg := []byte(msg)
		err := json.Unmarshal(bmsg, &m)
		if err != nil {
			return err
		}
		signed := &m

		c, _, err := GetPorcelainAPI(env).SignedMessageSend(
			req.Context,
			signed,
		)
		if err != nil {
			return err
		}

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: gas.NewGas(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
}

// WaitResult is the result of a message wait call.
type WaitResult struct {
	Message   *types.SignedMessage
	Receipt   *vm.MessageReceipt
	Signature vm.ActorMethodSignature
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
		cmdkit.Uint64Option("lookback", "Number of previous tipsets to be checked before waiting").WithDefault(msg.DefaultMessageWaitLookback),
		cmdkit.StringOption("timeout", "Maximum time to wait for message. e.g., 300ms, 1.5h, 2h45m.").WithDefault("10m"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}

		fmt.Printf("waiting for: %s\n", req.Arguments[0])

		found := false

		timeoutDuration, err := time.ParseDuration(req.Options["timeout"].(string))
		if err != nil {
			return errors.Wrap(err, "Invalid timeout string")
		}

		lookback, _ := req.Options["lookback"].(uint64)

		ctx, cancel := context.WithTimeout(req.Context, timeoutDuration)
		defer cancel()

		err = GetPorcelainAPI(env).MessageWait(ctx, msgCid, lookback, func(blk *block.Block, msg *types.SignedMessage, receipt *vm.MessageReceipt) error {
			found = true
			sig, err := GetPorcelainAPI(env).ActorGetSignature(req.Context, msg.Message.To, msg.Message.Method)
			if err != nil && err != cst.ErrNoMethod && err != cst.ErrNoActorImpl {
				return errors.Wrap(err, "Couldn't get signature for message")
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
}

// MessageStatusResult is the status of a message on chain or in the message queue/pool
type MessageStatusResult struct {
	InPool    bool // Whether the message is found in the mpool
	PoolMsg   *types.SignedMessage
	InOutbox  bool // Whether the message is found in the outbox
	OutboxMsg *message.Queued
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

		return re.Emit(&result)
	},
	Type: &MessageStatusResult{},
}
