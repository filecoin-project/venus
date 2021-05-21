package cmd

import (
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var showCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get human-readable representations of filecoin objects",
	},
	Subcommands: map[string]*cmds.Command{
		"block":    showBlockCmd,
		"header":   showHeaderCmd,
		"message":  showMessageCmd,
		"messages": showMessagesCmd,
		"receipts": showReceiptsCmd,
	},
}

var showBlockCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a full filecoin block by its header CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of block to show"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("messages", "m", "show messages in block"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		block, err := env.(*node.Env).ChainAPI.GetFullBlock(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: types.FullBlock{},
}

var showHeaderCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin block header by its CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		block, err := env.(*node.Env).ChainAPI.ChainGetBlock(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: types.BlockHeader{},
}

var showMessageCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin message by its CID",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		msg, err := env.(*node.Env).ChainAPI.ChainGetMessage(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(msg)
	},
	Type: types.UnsignedMessage{},
}

var showMessagesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin message collection by txmeta CID",
		ShortDescription: `Prints info for all messages in a collection,
at the given CID.  This CID is found in the "Messages" field of
the filecoin block header.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of message collection to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		bmsg, err := env.(*node.Env).ChainAPI.ChainGetBlockMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(bmsg)
	},
	Type: &apitypes.BlockMessages{},
}

var showReceiptsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin receipt collection by its CID",
		ShortDescription: `Prints info for all receipts in a collection,
at the given CID.  MessageReceipt collection CIDs are found in the "ParentMessageReceipts"
field of the filecoin block header.`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of receipt collection to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		receipts, err := env.(*node.Env).ChainAPI.ChainGetReceipts(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(receipts)
	},
	Type: []types.MessageReceipt{},
}
