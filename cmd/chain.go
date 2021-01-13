// Package commands implements the command to print the blockchain.
package cmd

import (
	"bytes"
	"os"
	"strconv"

	"github.com/filecoin-project/venus/app/node"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"export":   storeExportCmd,
		"head":     storeHeadCmd,
		"ls":       storeLsCmd,
		"status":   storeStatusCmd,
		"set-head": storeSetHeadCmd,
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
		head, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
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

		return re.Emit(&ChainHeadResult{Height: h, ParentWeight: pw, Cids: head.Key().Cids()})
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

		var err error
		height, _ := req.Options["height"].(int64)
		startTs, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}
		if height >= 0 {
			startTs, err = env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(req.Context, abi.ChainEpoch(height), startTs.Key())
			if err != nil {
				return err
			}
		}

		tipSetKeys, err := env.(*node.Env).ChainAPI.ChainList(req.Context, startTs.Key(), int(count))
		if err != nil {
			return err
		}

		for _, tipset := range tipSetKeys {
			if err := re.Emit(tipset.Cids()); err != nil {
				return err
			}
		}
		return nil
	},
	Type: []block.Block{},
}

type SyncTarget struct {
	TargetTs block.TipSetKey
	Height   abi.ChainEpoch
	State    string
}

type SyncStatus struct {
	Target []SyncTarget
}

var storeStatusCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show status of chain sync operation.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		//TODO give each target a status
		//syncStatus.Status = env.(*node.Env).SyncerAPI.SyncerStatus()
		targets := env.(*node.Env).SyncerAPI.SyncerTracker().Buckets()
		w := bytes.NewBufferString("")
		writer := NewSilentWriter(w)
		for index, t := range targets {
			writer.Println("Target:", strconv.Itoa(index+1))
			writer.Println("\tHeight:", strconv.FormatUint(uint64(t.Head.EnsureHeight()), 10))
			writer.Println("\tTipSet:", t.Head.Key().String())
			if t.InSyncing {
				writer.Println("\tStatus:Syncing")
			} else {
				writer.Println("\tStatus:Wait")
			}
		}
		if err := re.Emit(w); err != nil {
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
		return env.(*node.Env).ChainAPI.ChainSetHead(req.Context, maybeNewHead)
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

		if err := env.(*node.Env).ChainAPI.ChainExport(req.Context, expKey, f); err != nil {
			return err
		}
		return nil
	},
}
