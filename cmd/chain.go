// Package commands implements the command to print the blockchain.
package cmd

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/types"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"head":     chainHeadCmd,
		"ls":       chainLsCmd,
		"set-head": chainSetHeadCmd,
		"getblock": chainGetBlockCmd,
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

		h, err := head.Height()
		if err != nil {
			return err
		}

		pw, err := head.ParentWeight()
		if err != nil {
			return err
		}

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

		res := make([]ChainLsResult, 0)
		for _, key := range tipSetKeys {
			tp, err := env.(*node.Env).ChainAPI.ChainGetTipSet(key)
			if err != nil {
				return err
			}

			h, err := tp.Height()
			if err != nil {
				return err
			}

			strTt := time.Unix(int64(tp.MinTimestamp()), 0).Format("2006-01-02 15:04:05")

			blks := make([]BlockResult, len(tp.Blocks()))
			for idx, blk := range tp.Blocks() {
				blks[idx] = BlockResult{Cid: blk.Cid(), Miner: blk.Miner}
			}

			lsRes := ChainLsResult{Height: h, Timestamp: strTt, Blocks: blks}
			res = append(res, lsRes)
		}

		if err := re.Emit(res); err != nil {
			return err
		}
		return nil
	},
	Type: []ChainLsResult{},
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
			return xerrors.Errorf("get block failed: %w", err)
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
			return xerrors.Errorf("failed to get messages: %v", err)
		}

		pmsgs, err := env.(*node.Env).ChainAPI.ChainGetParentMessages(ctx, bcid)
		if err != nil {
			return xerrors.Errorf("failed to get parent messages: %v", err)
		}

		recpts, err := env.(*node.Env).ChainAPI.ChainGetParentReceipts(ctx, bcid)
		if err != nil {
			log.Warn(err)
		}

		cblock := struct {
			types.BlockHeader
			BlsMessages    []*types.UnsignedMessage
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

func apiMsgCids(in []chain.Message) []cid.Cid {
	out := make([]cid.Cid, len(in))
	for k, v := range in {
		out[k] = v.Cid
	}
	return out
}
