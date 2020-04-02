package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"

	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
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
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *block.FullBlock) error {
			wStr := block.Header.ParentWeight.String()
			_, err := fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Messages:  %s
Timestamp:  %s
`,
				block.Header.Miner,
				wStr,
				strconv.FormatInt(int64(block.Header.Height), 10),
				block.Header.Messages.String(),
				strconv.FormatUint(block.Header.Timestamp, 10),
			)
			if err != nil {
				return err
			}

			showMessages, _ := req.Options["messages"].(bool)
			if showMessages == true {
				_, err = fmt.Fprintf(w, `BLS Messages:\n  %s`+"\n", block.BLSMessages)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(w, `SECP Messages:\n  %s`+"\n", block.SECPMessages)
				if err != nil {
					return err
				}
			}
			return nil
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
	Type: block.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, block *block.Block) error {
			wStr := block.ParentWeight.String()
			_, err := fmt.Fprintf(w, `Block Details
Miner:  %s
Weight: %s
Height: %s
Timestamp:  %s
`,
				block.Miner,
				wStr,
				strconv.FormatInt(int64(block.Height), 10),
				strconv.FormatUint(block.Timestamp, 10),
			)
			return err
		}),
	},
}

type allMessages struct {
	BLS  []*types.UnsignedMessage
	SECP []*types.SignedMessage
}

var showMessagesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show a filecoin message collection by txmeta CID",
		ShortDescription: `Prints info for all messages in a collection,
at the given CID.  This CID is found in the "Messages" field of
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

		bls, secp, err := GetPorcelainAPI(env).ChainGetMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(&allMessages{BLS: bls, SECP: secp})
	},
	Type: &allMessages{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, messages *allMessages) error {
			outStr := "BLS messages \n"
			for _, msg := range messages.BLS {
				outStr += msg.String() + "\n"
			}
			outStr += "\nSECP messages \n"
			for _, msg := range messages.SECP {
				outStr += msg.String() + "\n"
			}
			_, err := fmt.Fprint(w, outStr)
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
	Type: []vm.MessageReceipt{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, receipts []vm.MessageReceipt) error {
			outStr := "Receipt Details\n"
			for _, r := range receipts {
				outStr += r.String() + "\n"
			}
			_, err := fmt.Fprint(w, outStr)
			return err
		}),
	},
}
