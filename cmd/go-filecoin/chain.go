// Package commands implements the command to print the blockchain.
package commands

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/libp2p/go-libp2p-core/peer"
)

var chainCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"export":   storeExportCmd,
		"head":     storeHeadCmd,
		"import":   storeImportCmd,
		"ls":       storeLsCmd,
		"status":   storeStatusCmd,
		"set-head": storeSetHeadCmd,
		"sync":     storeSyncCmd,
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
	Type: []block.Block{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *[]block.Block) error {
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
		syncStatus := GetPorcelainAPI(env).SyncerStatus()
		if err := re.Emit(syncStatus); err != nil {
			return err
		}
		return nil
	},
}

var storeSetHeadCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Set the chain head to a specific tipset key.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cids", true, true, "CID's of the blocks of the tipset to set the chain head to."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		headCids, err := cidsFromSlice(req.Arguments)
		if err != nil {
			return err
		}
		maybeNewHead := block.NewTipSetKey(headCids...)
		return GetPorcelainAPI(env).ChainSetHead(req.Context, maybeNewHead)
	},
}

var storeSyncCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Instruct the chain syncer to sync a specific chain head, going to network if required.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerid", true, false, "Base58-encoded libp2p peer ID to sync from"),
		cmdkit.StringArg("cids", true, true, "CID's of the blocks of the tipset to sync."),
	},
	Options: []cmdkit.Option{},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		syncPid, err := peer.IDB58Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		syncCids, err := cidsFromSlice(req.Arguments[1:])
		if err != nil {
			return err
		}

		syncKey := block.NewTipSetKey(syncCids...)
		ci := &block.ChainInfo{
			Source: syncPid,
			Sender: syncPid,
			Height: 0, // only checked when trusted is false.
			Head:   syncKey,
		}
		return GetPorcelainAPI(env).ChainSyncHandleNewTipSet(ci)
	},
}

var storeExportCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Export the chain store to a car file.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("file", true, false, "File to export chain data to."),
		cmdkit.StringArg("cids", true, true, "CID's of the blocks of the tipset to export from."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		f, err := os.Create(req.Arguments[0])
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		expCids, err := cidsFromSlice(req.Arguments[1:])
		if err != nil {
			return err
		}
		expKey := block.NewTipSetKey(expCids...)

		if err := GetPorcelainAPI(env).ChainExport(req.Context, expKey, f); err != nil {
			return err
		}
		return nil
	},
}

var storeImportCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Import the chain from a car file.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("file", true, false, "File to import chain data from.").EnableStdin(),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		iter := req.Files.Entries()
		if !iter.Next() {
			return fmt.Errorf("no file given: %s", iter.Err())
		}

		fi, ok := iter.Node().(files.File)
		if !ok {
			return fmt.Errorf("given file was not a files.File")
		}
		defer func() { _ = fi.Close() }()
		headKey, err := GetPorcelainAPI(env).ChainImport(req.Context, fi)
		if err != nil {
			return err
		}
		return re.Emit(headKey)
	},
}
