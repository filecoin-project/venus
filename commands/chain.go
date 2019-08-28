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
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head":        storeHeadCmd,
		"ls":          storeLsCmd,
		"sync-status": syncStatusCmd,
		"sync-tipset": syncTipSetCmd,
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

var syncStatusCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show status of chain sync operation.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		syncStatus := GetPorcelainAPI(env).ChainSyncStatus()
		if err := re.Emit(syncStatus); err != nil {
			return err
		}
		return nil
	},
}

var syncTipSetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Sync and validate a chain.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerid", true, false, "Base58-encoded libp2p peer ID that a chain will be synced from"),
		cmdkit.StringArg("cid", true, false, "CID of the TipSet to sync"),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("trusted", "Trust the TipSet when syncing").WithDefault(true),
		cmdkit.Uint64Option("height", "Height of the TipSet to sync").WithDefault(0),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		peer, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		tsCid, err := cid.Parse(req.Arguments[1])
		if err != nil {
			return errors.Wrap(err, "invalid cid "+req.Arguments[0])
		}
		head := types.NewTipSetKey(tsCid)

		trusted := req.Options["trusted"].(bool)
		height, err := strconv.ParseUint(req.Options["height"].(string), 10, 64)
		if err != nil {
			return err
		}
		ci := types.NewChainInfo(peer, head, height)
		syncStatus := GetPorcelainAPI(env).ChainSyncHandleNewTipSet(req.Context, ci, trusted)
		if err := re.Emit(syncStatus); err != nil {
			return err
		}
		return nil
	},
}
