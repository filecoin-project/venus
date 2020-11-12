package commands

import (
	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/types"

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

		block, err := GetPorcelainAPI(env).ChainGetFullBlock(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: block.FullBlock{},
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

		block, err := GetPorcelainAPI(env).ChainGetBlock(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(block)
	},
	Type: block.Block{},
}

type allMessages struct {
	BLS  []*types.UnsignedMessage
	SECP []*types.SignedMessage
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

		bls, secp, err := GetPorcelainAPI(env).ChainGetMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(&allMessages{BLS: bls, SECP: secp})
	},
	Type: &allMessages{},
}

var showReceiptsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin receipt collection by its CID",
		ShortDescription: `Prints info for all receipts in a collection,
at the given CID.  MessageReceipt collection CIDs are found in the "MessageReceipts"
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

		receipts, err := GetPorcelainAPI(env).ChainGetReceipts(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(receipts)
	},
	Type: []types.MessageReceipt{},
}
