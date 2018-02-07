package commands

import (
	"encoding/json"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/types"
)

var sendMsgCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Send a message", // This feels too generic...
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("target", true, false, "address to send message to"),
		cmdkit.StringArg("method", true, false, "method to invoke"),
	},
	Options: []cmdkit.Option{
		cmdkit.IntOption("value", "value to send with message"),
		cmdkit.StringOption("from", "address to send message from"),
		cmdkit.BoolOption("offchain", "send the message without adding to a block"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		target, err := types.ParseAddress(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		method := req.Arguments[1]

		var val int
		val, _ = req.Options["value"].(int)
		offchain := req.Options["offchain"].(bool)
		from, _ := req.Options["from"].(string)

		var fromAddr types.Address
		if from != "" {
			fromAddr, err = types.ParseAddress(from)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
		} else {
			addrs := n.Wallet.GetAddresses()
			if len(addrs) == 0 {
				re.SetError("no addresses in local wallet", cmdkit.ErrNormal)
				return
			}
			fromAddr = addrs[0]
		}

		msg := types.NewMessage(fromAddr, target, big.NewInt(int64(val)), method, nil)

		if offchain {
			// fetch state tree
			fcn := GetNode(env)
			blk := fcn.ChainMgr.GetBestBlock()
			if blk.StateRoot == nil {
				re.SetError("state root in latest block was nil", cmdkit.ErrNormal)
				return
			}

			tree, err := types.LoadStateTree(req.Context, fcn.CborStore, blk.StateRoot)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			receipt, err := core.ApplyMessage(req.Context, tree, msg)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			re.Emit(receipt) // nolint: errcheck
		} else {
			if err := n.AddNewMessage(req.Context, msg); err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			c, err := msg.Cid()
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}

			re.Emit(&types.MessageReceipt{Message: c}) // nolint: errcheck
		}
	},
	Type: &types.MessageReceipt{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, receipt *types.MessageReceipt) error {
			marshaled, err := json.MarshalIndent(receipt, "", "\t")
			if err != nil {
				return err
			}
			_, err = w.Write(marshaled)
			return err
		}),
	},
}
