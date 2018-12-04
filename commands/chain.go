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
		Tagline: "get heaviest tipset CIDs",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		out, err := GetAPI(env).Chain().Head()
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		re.Emit(out) //nolint: errcheck
	},
	Type: []*cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "list blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("long", "l", "list blocks in long format, including CID, Miner, StateRoot, block height and message count respectively"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		for raw := range GetAPI(env).Chain().Ls(req.Context) {
			switch v := raw.(type) {
			case error:
				re.SetError(v, cmdkit.ErrNormal)
				return
			case consensus.TipSet:
				if len(v) == 0 {
					panic("tipsets from this channel should have at least one member")
				}
				re.Emit(v.ToSlice()) // nolint: errcheck
			default:
				re.SetError("unexpected type", cmdkit.ErrNormal)
				return
			}
		}
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
