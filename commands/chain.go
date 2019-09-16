// Package commands implements the command to print the blockchain.
package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmdkit"
	"github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/go-filecoin/chain"
	"github.com/filecoin-project/go-filecoin/types"
)

type ExecutedMessage struct {
	Message *types.SignedMessage
	Receipt *types.MessageReceipt
}

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head":      storeHeadCmd,
		"ls":        storeLsCmd,
		"status":    storeStatusCmd,
		"list-msgs": storeLsMsgsCmd,
	},
}

var storeHeadCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get heaviest tipset CIDs",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		head, err := GetPorcelainAPI(env).ChainHead()
		if err != nil {
			return err
		}
		return re.Emit(head.Key())
	},
	Type: []cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res []cid.Cid) error {
			for _, r := range res {
				_, err := fmt.Fprintln(w, r.String())
				if err != nil {
					return err
				}
			}
			return nil
		}),
	},
}

var storeLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("long", "l", "List blocks in long format, including CID, Miner, StateRoot, block height and message count respectively"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		iter, err := GetPorcelainAPI(env).ChainLs(req.Context)
		if err != nil {
			return err
		}
		for ; !iter.Complete(); err = iter.Next() {
			if err != nil {
				return err
			}
			if !iter.Value().Defined() {
				panic("tipsets from this iterator should have at least one member")
			}
			if err := re.Emit(iter.Value().ToSlice()); err != nil {
				return err
			}
		}
		return nil
	},
	Type: []types.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *[]types.Block) error {
			showAll, _ := req.Options["long"].(bool)
			blocks := *res

			for _, block := range blocks {
				var output strings.Builder

				if showAll {
					output.WriteString(block.Cid().String())
					output.WriteString("\t")
					output.WriteString(block.Miner.String())
					output.WriteString("\t")
					output.WriteString(block.StateRoot.String())
					output.WriteString("\t")
					output.WriteString(strconv.FormatUint(uint64(block.Height), 10))
					output.WriteString("\t")
					output.WriteString(block.Messages.String())
				} else {
					output.WriteString(block.Cid().String())
				}

				_, err := fmt.Fprintln(w, output.String())
				if err != nil {
					return err
				}
			}

			return nil
		}),
	},
}

var storeStatusCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show status of chain sync operation.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		syncStatus := GetPorcelainAPI(env).ChainStatus()
		if err := re.Emit(syncStatus); err != nil {
			return err
		}
		return nil
	},
}

var storeLsMsgsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List messages in the blockchain",
		ShortDescription: `Provides a list of messages in order from genesis to head`,
	},
	Options: []cmdkit.Option{
		//cmdkit.BoolOption("long", "l", "List messages in long format (default: only CID)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		iter, err := GetPorcelainAPI(env).ChainLs(req.Context)
		if err != nil {
			return err
		}
		// Accumulate tipsets in descending order.
		var tips []types.TipSet
		for ; !iter.Complete(); err = iter.Next() {
			if err != nil {
				return err
			}
			tips = append(tips, iter.Value())
		}
		// Re-order into ascending order.
		chain.Reverse(tips)

		// BEWARE: the chain doesn't provide the true receipt status of messages since, in a
		// multi-block tipset, the context for execution is not the tipset's parent state for blocks
		// after the first in a tipset.
		// A correct version would need to re-execute the messages.
		for _, tip := range tips {
			for i := 0; i < tip.Len(); i++ {
				msgs, err := GetPorcelainAPI(env).ChainGetMessages(req.Context, tip.At(i).Messages)
				if err != nil {
					return err
				}
				rcps, err := GetPorcelainAPI(env).ChainGetReceipts(req.Context, tip.At(i).MessageReceipts)
				if err != nil {
					return err
				}
				for j := 0; j < len(msgs); j++ {
					_ = re.Emit(&ExecutedMessage{
						Message: msgs[j],
						Receipt: rcps[j],
					})
				}
			}
		}

		return nil
	},
	Type: ExecutedMessage{},
	Encoders: cmds.EncoderMap{},
}
