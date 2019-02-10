// Package commands implements the command to print the blockchain.
package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qma6uuSyjkecGhMFFLfzyJDPyoDtNJSHJNweDccZhaWkgU/go-ipfs-cmds"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/porcelain"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head": chainHeadCmd,
		"ls":   chainLsCmd,
	},
}

var chainHeadCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get heaviest tipset CIDs",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ts := GetPorcelainAPI(env).ChainHead(req.Context)

		// Sort CIDs for a stable order
		tsSlice := ts.ToSlice()
		out := types.SortedCidSet{}
		for _, b := range tsSlice {
			out.Add(b.Cid())
		}

		return re.Emit(out)
	},
	Type: []cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "List blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("long", "l", "List blocks in long format, including CID, Miner, StateRoot, block height and message count respectively"),
		cmdkit.Uint64Option("since", "s", "Show only blocks including and past the provided height. By default, will include all blocks from head to genesis."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		since, ok := req.Options["since"].(uint64)
		if !ok {
			since = 0
		}

		porcelainApi := GetPorcelainAPI(env)
		for raw := range porcelain.ChainBlocksSince(req.Context, porcelainApi, since) {
			switch v := raw.(type) {
			case error:
				return v
			case consensus.TipSet:
				if len(v) == 0 {
					panic("tipsets from this channel should have at least one member")
				}
				if err := re.Emit(v.ToSlice()); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unexpected type")
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
					output.WriteString(strconv.Itoa(len(block.Messages)))
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
