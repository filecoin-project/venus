package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/vm"

	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	"github.com/filecoin-project/venus/app/submodule/messaging/msg"
	"github.com/filecoin-project/venus/pkg/message"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
)

var msgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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
	GasUsed types.Unit
	Preview bool
}

var msgSendCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send a message", // This feels too generic...
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("target", true, false, "RustFulAddress of the actor to send the message to"),
		cmds.StringArg("method", false, false, "The method to invoke on the target actor"),
	},
	Options: []cmds.Option{
		cmds.StringOption("value", "Value to send with message in FIL"),
		cmds.StringOption("from", "RustFulAddress to send message from"),
		feecapOption,
		premiumOption,
		limitOption,
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

		feecap, premium, gasLimit, err := parseGasOptions(req)
		if err != nil {
			return err
		}

		methodID := builtin.MethodSend
		methodInput, ok := req.Options["method"].(uint64)
		if ok {
			methodID = abi.MethodNum(methodInput)
		}

		c, err := env.(*node.Env).MessagingAPI.MessageSend(
			req.Context,
			fromAddr,
			target,
			val,
			feecap,
			premium,
			gasLimit,
			methodID,
			[]byte{},
		)
		if err != nil {
			return err
		}

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: types.NewGas(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
}

var signedMsgSendCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Send a signed message",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("message", true, false, "Signed Json message"),
	},
	Options: []cmds.Option{},

	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msg := req.Arguments[0]

		m := types.SignedMessage{}

		bmsg := []byte(msg)
		err := json.Unmarshal(bmsg, &m)
		if err != nil {
			return err
		}
		signed := &m

		c, err := env.(*node.Env).MessagingAPI.SignedMessageSend(
			req.Context,
			signed,
		)
		if err != nil {
			return err
		}

		return re.Emit(&MessageSendResult{
			Cid:     c,
			GasUsed: types.NewGas(0),
			Preview: false,
		})
	},
	Type: &MessageSendResult{},
}

// WaitResult is the result of a message wait call.
type WaitResult struct {
	Message   *types.UnsignedMessage
	Receipt   *types.MessageReceipt
	Signature vm.ActorMethodSignature
}

var msgWaitCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Wait for a message to appear in a mined block",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of the message to wait for"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("message", "Print the whole message").WithDefault(true),
		cmds.BoolOption("receipt", "Print the whole message receipt").WithDefault(true),
		cmds.BoolOption("return", "Print the return value from the receipt").WithDefault(false),
		cmds.Uint64Option("confidence", "Number of block to confirm message").WithDefault(constants.DefaultConfidence),
		cmds.Uint64Option("lookback", "Number of previous tipsets to be checked before waiting").WithDefault(constants.DefaultMessageWaitLookback),
		cmds.StringOption("timeout", "Maximum time to wait for message. e.g., 300ms, 1.5h, 2h45m.").WithDefault("10m"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}
		fmt.Printf("waiting for: %s\n", req.Arguments[0])

		timeoutDuration, err := time.ParseDuration(req.Options["timeout"].(string))
		if err != nil {
			return errors.Wrap(err, "Invalid timeout string")
		}

		lookback, _ := req.Options["lookback"].(uint64)
		confidence, _ := req.Options["confidence"].(uint64)
		ctx, cancel := context.WithTimeout(req.Context, timeoutDuration)
		defer cancel()

		chainmsg, err := env.(*node.Env).MessagingAPI.MessageWait(ctx, msgCid, abi.ChainEpoch(confidence), abi.ChainEpoch(lookback))
		if err != nil {
			return err
		}
		if chainmsg != nil {
			sig, err := env.(*node.Env).ChainAPI.ActorGetSignature(req.Context, chainmsg.Message.VMMessage().To, chainmsg.Message.VMMessage().Method)
			if err != nil && err != cst.ErrNoMethod && err != cst.ErrNoActorImpl {
				return errors.Wrap(err, "Couldn't get signature for message")
			}

			res := WaitResult{
				Message: chainmsg.Message.VMMessage(),
				Receipt: chainmsg.Receipt,
				// Signature is required to decode the output.
				Signature: sig,
			}
			re.Emit(&res) // nolint: errcheck
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
	Helptext: cmds.HelpText{
		Tagline: "Show status of a message",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of the message to inspect"),
	},
	Options: []cmds.Option{},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		msgCid, err := cid.Parse(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}

		api := env.(*node.Env).MessagingAPI
		result := MessageStatusResult{}

		// Look in message pool
		result.PoolMsg, err = api.MessagePoolGet(msgCid)
		result.InOutbox = err == nil

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
