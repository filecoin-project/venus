// Package commands implements the command to print the blockchain.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/constants"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head":         chainHeadCmd,
		"ls":           chainLsCmd,
		"set-head":     chainSetHeadCmd,
		"getblock":     chainGetBlockCmd,
		"get-message":  chainGetMessageCmd,
		"get-messages": chainGetMessagesCmd,
		"get-receipts": chainGetReceiptsCmd,
		"disputer":     chainDisputeSetCmd,
		"export":       chainExportCmd,
	},
}

type ChainHeadResult struct {
	Height       abi.ChainEpoch
	ParentWeight big.Int
	Cids         []cid.Cid
	Timestamp    string
}

var chainHeadCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get heaviest tipset info",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		head, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		h := head.Height()
		pw := head.ParentWeight()

		strTt := time.Unix(int64(head.MinTimestamp()), 0).Format("2006-01-02 15:04:05")

		return re.Emit(&ChainHeadResult{Height: h, ParentWeight: pw, Cids: head.Key().Cids(), Timestamp: strTt})
	},
	Type: &ChainHeadResult{},
}

type BlockResult struct {
	Cid   cid.Cid
	Miner address.Address
}

type ChainLsResult struct {
	Height    abi.ChainEpoch
	Timestamp string
	Blocks    []BlockResult
}

var chainLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "List blocks in the blockchain",
		ShortDescription: `Provides a list of blocks in order from head to genesis. By default, only CIDs are returned for each block.`,
	},
	Options: []cmds.Option{
		cmds.Int64Option("height", "Start height of the query").WithDefault(int64(-1)),
		cmds.UintOption("count", "Number of queries").WithDefault(uint(10)),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		count, _ := req.Options["count"].(uint)
		if count < 1 {
			return nil
		}

		var err error

		startTS, err := env.(*node.Env).ChainAPI.ChainHead(req.Context)
		if err != nil {
			return err
		}

		height, _ := req.Options["height"].(int64)
		if height >= 0 && abi.ChainEpoch(height) < startTS.Height() {
			startTS, err = env.(*node.Env).ChainAPI.ChainGetTipSetByHeight(req.Context, abi.ChainEpoch(height), startTS.Key())
			if err != nil {
				return err
			}
		}

		if abi.ChainEpoch(count) > startTS.Height()+1 {
			count = uint(startTS.Height() + 1)
		}
		tipSetKeys, err := env.(*node.Env).ChainAPI.ChainList(req.Context, startTS.Key(), int(count))
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)
		tpInfoStr := ""
		for _, key := range tipSetKeys {
			tp, err := env.(*node.Env).ChainAPI.ChainGetTipSet(req.Context, key)
			if err != nil {
				return err
			}

			strTt := time.Unix(int64(tp.MinTimestamp()), 0).Format("2006-01-02 15:04:05")

			oneTpInfoStr := fmt.Sprintf("%v: (%s) [ ", tp.Height(), strTt)
			for _, blk := range tp.Blocks() {
				oneTpInfoStr += fmt.Sprintf("%s: %s,", blk.Cid().String(), blk.Miner)
			}
			oneTpInfoStr += " ]"

			tpInfoStr += oneTpInfoStr + "\n"
		}

		writer.WriteString(tpInfoStr)

		return re.Emit(buf)
	},
}

var chainSetHeadCmd = &cmds.Command{
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
		maybeNewHead := types.NewTipSetKey(headCids...)
		return env.(*node.Env).ChainAPI.ChainSetHead(req.Context, maybeNewHead)
	},
}

var chainGetBlockCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a block and print its details.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CID of the block to show."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("raw", "print just the raw block header"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		bcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx := req.Context
		blk, err := env.(*node.Env).ChainAPI.ChainGetBlock(ctx, bcid)
		if err != nil {
			return fmt.Errorf("get block failed: %w", err)
		}

		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		if _, ok := req.Options["raw"].(bool); ok {
			out, err := json.MarshalIndent(blk, "", "  ")
			if err != nil {
				return err
			}

			_ = writer.Write(out)

			return re.Emit(buf)
		}

		msgs, err := env.(*node.Env).ChainAPI.ChainGetBlockMessages(ctx, bcid)
		if err != nil {
			return fmt.Errorf("failed to get messages: %v", err)
		}

		pmsgs, err := env.(*node.Env).ChainAPI.ChainGetParentMessages(ctx, bcid)
		if err != nil {
			return fmt.Errorf("failed to get parent messages: %v", err)
		}

		recpts, err := env.(*node.Env).ChainAPI.ChainGetParentReceipts(ctx, bcid)
		if err != nil {
			log.Warn(err)
		}

		cblock := struct {
			types.BlockHeader
			BlsMessages    []*types.Message
			SecpkMessages  []*types.SignedMessage
			ParentReceipts []*types.MessageReceipt
			ParentMessages []cid.Cid
		}{}

		cblock.BlockHeader = *blk
		cblock.BlsMessages = msgs.BlsMessages
		cblock.SecpkMessages = msgs.SecpkMessages
		cblock.ParentReceipts = recpts
		cblock.ParentMessages = apiMsgCids(pmsgs)

		out, err := json.MarshalIndent(cblock, "", "  ")
		if err != nil {
			return err
		}

		_ = writer.Write(out)

		return re.Emit(buf)
	},
}

var chainGetMessageCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin message by its CID",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of message to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		msg, err := env.(*node.Env).ChainAPI.ChainGetMessage(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(msg)
	},
	Type: types.Message{},
}

var chainGetMessagesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show a filecoin message collection by block CID",
		ShortDescription: "Prints info for all messages in a collection, at the given block CID.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID of block to show"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		bmsg, err := env.(*node.Env).ChainAPI.ChainGetBlockMessages(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(bmsg)
	},
	Type: &types.BlockMessages{},
}

var chainGetReceiptsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a filecoin receipt collection by its CID",
		ShortDescription: `Prints info for all receipts in a collection,
at the given CID.  MessageReceipt collection CIDs are found in the "ParentMessageReceipts"
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

		receipts, err := env.(*node.Env).ChainAPI.ChainGetReceipts(req.Context, cid)
		if err != nil {
			return err
		}

		return re.Emit(receipts)
	},
	Type: []types.MessageReceipt{},
}

func apiMsgCids(in []types.MessageCID) []cid.Cid {
	out := make([]cid.Cid, len(in))
	for k, v := range in {
		out[k] = v.Cid
	}
	return out
}

var chainExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "export chain to a car file",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("outputPath", true, false, ""),
	},
	Options: []cmds.Option{
		cmds.StringOption("tipset").WithDefault(""),
		cmds.Int64Option("recent-stateroots", "specify the number of recent state roots to include in the export").WithDefault(int64(0)),
		cmds.BoolOption("skip-old-msgs").WithDefault(false),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 1 {
			return errors.New("must specify filename to export chain to")
		}

		rsrs := abi.ChainEpoch(req.Options["recent-stateroots"].(int64))
		if rsrs > 0 && rsrs < constants.Finality {
			return fmt.Errorf("\"recent-stateroots\" has to be greater than %d", constants.Finality)
		}

		fi, err := os.Create(req.Arguments[0])
		if err != nil {
			return err
		}
		defer func() {
			err := fi.Close()
			if err != nil {
				fmt.Printf("error closing output file: %+v", err)
			}
		}()

		ts, err := LoadTipSet(req.Context, req, env.(*node.Env).ChainAPI)
		if err != nil {
			return err
		}

		skipold := req.Options["skip-old-msgs"].(bool)

		if rsrs == 0 && skipold {
			return fmt.Errorf("must pass recent stateroots along with skip-old-msgs")
		}

		stream, err := env.(*node.Env).ChainAPI.ChainExport(req.Context, rsrs, skipold, ts.Key())
		if err != nil {
			return err
		}

		var last bool
		for b := range stream {
			last = len(b) == 0

			_, err := fi.Write(b)
			if err != nil {
				return err
			}
		}

		if !last {
			return fmt.Errorf("incomplete export (remote connection lost?)")
		}

		return nil
	},
}

// LoadTipSet gets the tipset from the context, or the head from the API.
//
// It always gets the head from the API so commands use a consistent tipset even if time pases.
func LoadTipSet(ctx context.Context, req *cmds.Request, chainAPI v1api.IChain) (*types.TipSet, error) {
	tss := req.Options["tipset"].(string)
	if tss == "" {
		return chainAPI.ChainHead(ctx)
	}

	return ParseTipSetRef(ctx, chainAPI, tss)
}

func ParseTipSetRef(ctx context.Context, chainAPI v1api.IChain, tss string) (*types.TipSet, error) {
	if tss[0] == '@' {
		if tss == "@head" {
			return chainAPI.ChainHead(ctx)
		}

		var h uint64
		if _, err := fmt.Sscanf(tss, "@%d", &h); err != nil {
			return nil, fmt.Errorf("parsing height tipset ref: %w", err)
		}

		return chainAPI.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(h), types.EmptyTSK)
	}

	cids, err := ParseTipSetString(tss)
	if err != nil {
		return nil, err
	}

	if len(cids) == 0 {
		return nil, nil
	}

	k := types.NewTipSetKey(cids...)
	ts, err := chainAPI.ChainGetTipSet(ctx, k)
	if err != nil {
		return nil, err
	}

	return ts, nil
}

func ParseTipSetString(ts string) ([]cid.Cid, error) {
	strs := strings.Split(ts, ",")

	var cids []cid.Cid
	for _, s := range strs {
		c, err := cid.Parse(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		cids = append(cids, c)
	}

	return cids, nil
}
