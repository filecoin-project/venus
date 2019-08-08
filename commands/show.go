package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/types"
)

var showCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
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
	Helptext: cmdkit.HelpText{
		Tagline: "Show a full filecoin block by its header CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of block to show"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("messages", "m", "show messages in block"),
		cmdkit.BoolOption("receipts", "r", "show receipts in block"),
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
	Type: types.FullBlock{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *types.FullBlock) error {
			wStr, err := types.FixedStr(uint64(block.Header.ParentWeight))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Nonce:  %s
Messages:  %s
Receipts:  %s
Timestamp:  %s
`,
				block.Header.Miner,
				wStr,
				strconv.FormatUint(uint64(block.Header.Height), 10),
				strconv.FormatUint(uint64(block.Header.Nonce), 10),
				block.Header.Messages.String(),
				block.Header.MessageReceipts.String(),
				strconv.FormatUint(uint64(block.Header.Timestamp), 10),
			)
			if err != nil {
				return err
			}

			showMessages, _ := req.Options["messages"].(bool)
			if showMessages == true {
				_, err = fmt.Fprintf(w, `Messages:  %s`+"\n", block.Messages)
			}
			if err != nil {
				return err
			}
			showReceipts, _ := req.Options["receipts"].(bool)
			if showReceipts == true {
				_, err = fmt.Fprintf(w, `Receipts: %s`+"\n", block.Receipts)
			}
			return err
		}),
	},
}

var showHeaderCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a filecoin block header by its CID",
		ShortDescription: `Prints the miner, parent weight, height,
and nonce of a given block. If JSON encoding is specified with the --enc flag,
all other block properties will be included as well.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of block to show"),
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
	Type: types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *types.Block) error {
			wStr, err := types.FixedStr(uint64(block.ParentWeight))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Nonce:  %s
Timestamp:  %s
`,
				block.Miner,
				wStr,
				strconv.FormatUint(uint64(block.Height), 10),
				strconv.FormatUint(uint64(block.Nonce), 10),
				strconv.FormatUint(uint64(block.Timestamp), 10),
			)
			return err
		}),
	},
}

var showMessagesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a filecoin message collection by its CID",
		ShortDescription: `Prints info for all messages in a collection,
at the given CID.  Message collection CIDs are found in the "Messages" field of 
the filecoin block header.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of message collection to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		messages, err := GetPorcelainAPI(env).ChainGetMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(messages)
	},
	Type: []*types.SignedMessage{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, messages []*types.SignedMessage) error {
			outStr := "Messages Details\n"
			for _, msg := range messages {
				outStr += msg.String() + "\n"
			}
			_, err := fmt.Fprintf(w, outStr)
			return err
		}),
	},
}

var showReceiptsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a filecoin receipt collection by its CID",
		ShortDescription: `Prints info for all receipts in a collection,
at the given CID.  Receipt collection CIDs are found in the "MessageReceipts"
field of the filecoin block header.`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, false, "CID of receipt collection to show"),
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
	Type: []*types.MessageReceipt{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, receipts []*types.MessageReceipt) error {
			outStr := "Receipt Details\n"
			for _, r := range receipts {
				outStr += r.String() + "\n"
			}
			_, err := fmt.Fprintf(w, outStr)
			return err
		}),
	},
}
