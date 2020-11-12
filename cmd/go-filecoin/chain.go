// Package commands implements the command to print the blockchain.
package commands

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

type ChainHeadResult struct {
	Height       abi.ChainEpoch
	ParentWeight big.Int
	Cids         []cid.Cid
}

var storeHeadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get heaviest tipset info",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		head, err := GetPorcelainAPI(env).ChainHead()
		if err != nil {
			return err
		}

		h, err := head.Height()
		if err != nil {
			return err
		}

		pw, err := head.ParentWeight()
		if err != nil {
			return err
		}

		return re.Emit(&ChainHeadResult{Height: h, ParentWeight: pw, Cids: head.Key().ToSlice()})
	},
	Type: &ChainHeadResult{},
}

var storeLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmds.Option{
		cmds.Int64Option("height", "Start height of the query").WithDefault(-1),
		cmds.UintOption("count", "Number of queries").WithDefault(10),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		count, _ := req.Options["count"].(uint)
		if count < 1 {
			return nil
		}

		var iter *chain.TipsetIterator
		var err error
		height, _ := req.Options["height"].(int64)
		if height >= 0 {
			ts, err := GetPorcelainAPI(env).ChainGetTipSetByHeight(req.Context, nil, abi.ChainEpoch(height), true)
			if err != nil {
				return err
			}

			iter, err = GetPorcelainAPI(env).ChainLsWithHead(req.Context, ts.Key())
			if err != nil {
				return err
			}
		} else {
			iter, err = GetPorcelainAPI(env).ChainLs(req.Context)
			if err != nil {
				return err
			}
		}

		var number uint = 0
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

			number++
			if number >= count {
				break
			}
		}
		return nil
	},
	Type: []block.Block{},
}

var storeStatusCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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
	Helptext: cmds.HelpText{
		Tagline: "Set the chain head to a specific tipset key.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cids", true, true, "CID's of the blocks of the tipset to set the chain head to."),
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
	Helptext: cmds.HelpText{
		Tagline: "Instruct the chain syncer to sync a specific chain head, going to network if required.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peerid", true, false, "Base58-encoded libp2p peer ID to sync from"),
		cmds.StringArg("cids", true, true, "CID's of the blocks of the tipset to sync."),
	},
	Options: []cmds.Option{},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		syncPid, err := peer.Decode(req.Arguments[0])
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
	Helptext: cmds.HelpText{
		Tagline: "Export the chain store to a car file.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("file", true, false, "File to export chain data to."),
		cmds.StringArg("cids", true, true, "CID's of the blocks of the tipset to export from."),
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
	Helptext: cmds.HelpText{
		Tagline: "Import the chain from a car file.",
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "File to import chain data from.").EnableStdin(),
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
