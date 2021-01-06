// Package commands implements the command to print the blockchain.
package cmd

import (
	"context"
	"fmt"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
	"os"
	"strings"

	"github.com/filecoin-project/venus/app/node"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/pkg/block"
)

var chainCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect the filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"export":     storeExportCmd,
		"head":       storeHeadCmd,
		"ls":         storeLsCmd,
		"status":     storeStatusCmd,
		"set-head":   storeSetHeadCmd,
		"sync":       storeSyncCmd,
		"getblock":   storeGetBlock,
		"getmessage": storeGetMsgCmd,
		"gasprice":   storeGasPriceCmd,
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

var storeStatusCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show status of chain sync operation.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		syncStatus := env.(*node.Env).SyncerAPI.SyncerStatus()
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
		return env.(*node.Env).ChainAPI.ChainSetHead(req.Context, maybeNewHead)
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
		return env.(*node.Env).SyncerAPI.ChainSyncHandleNewTipSet(ci)
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

var storeGetBlock = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a block and print its details.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("blockCid", true, false, "File to export chain data to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("raw", "print just the raw block header"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := context.TODO()

		if len(req.Arguments) == 0 {
			_ = re.Emit("must pass cid of block to print")
			return nil
		}

		bcid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		api := env.(*node.Env).ChainAPI
		blk, err := api.ChainGetBlock(ctx, bcid)
		if err != nil {
			return xerrors.Errorf("get block failed: %w", err)
		}

		raw := req.Options["raw"]
		if raw != nil && raw.(bool) {
			_ = re.Emit(blk)
			return nil
		}

		msgs, err := api.ChainGetBlockMessages(ctx, bcid)
		if err != nil {
			return xerrors.Errorf("failed to get messages: %w", err)
		}

		pmsgs, err := api.ChainGetParentMessages(ctx, bcid)
		if err != nil {
			return xerrors.Errorf("failed to get parent messages: %w", err)
		}

		recpts, err := api.ChainGetParentReceipts(ctx, bcid)
		if err != nil {
			logrus.Warn(err)
			//return xerrors.Errorf("failed to get receipts: %w", err)
		}

		cblock := struct {
			block.Block
			BlsMessages    []*types.UnsignedMessage
			SecpkMessages  []*types.SignedMessage
			ParentReceipts []types.MessageReceipt
			ParentMessages []cid.Cid
		}{}

		cblock.Block = *blk
		cblock.BlsMessages = msgs.BlsMessages
		cblock.SecpkMessages = msgs.SecpkMessages
		cblock.ParentReceipts = recpts
		cblock.ParentMessages = apiMsgCids(pmsgs)

		_ = re.Emit(cblock)
		return nil

	},
}

func apiMsgCids(in []*types.UnsignedMessage) []cid.Cid {
	out := make([]cid.Cid, len(in))
	for k, v := range in {
		id, _ := v.Cid()
		out[k] = id
	}
	return out
}

var storeGetMsgCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and print a message by its cid",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("messageCid", true, false, "must pass a cid of a message to get"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) == 0 {
			_ = re.Emit("must pass a cid of a message to get")
			return nil
		}

		c, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return xerrors.Errorf("failed to parse cid input: %w", err)
		}

		ctx := context.TODO()
		mb, err := env.(*node.Env).ChainAPI.ChainReadObj(ctx, c)
		if err != nil {
			return xerrors.Errorf("failed to read object: %w", err)
		}

		var i interface{}
		m, err := types.DecodeMessage(mb)
		if err != nil {
			sm, err := types.DecodeSignedMessage(mb)
			if err != nil {
				return xerrors.Errorf("failed to decode object as a message: %w", err)
			}
			i = sm
		} else {
			i = m
		}

		_ = re.Emit(i)
		return nil
	},
}

var storeSetHead = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "manually set the local nodes head tipset (Caution: normally only used for recovery)",
		ShortDescription: "manually set the local nodes head tipset (Caution: normally only used for recovery)",
	},
	Options: []cmds.Option{
		cmds.BoolOption("genesis", "reset head to genesis"),
		cmds.Uint64Option("epoch", "reset head to given epoch"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("tipsetkey", true, false, "format is {xxx,yyy,zzz}"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var ts *block.TipSet
		var err error
		ctx := context.TODO()

		genesis := req.Options["genesis"].(bool)
		epoch := req.Options["epoch"].(uint64)

		api := env.(*node.Env).ChainAPI
		if genesis {
			ts, err = api.ChainGetGenesis(ctx)

		}
		if ts == nil && epoch != 0 {
			ts, err = api.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(epoch), block.TipSetKey{})
		}
		if ts == nil {
			ts, err = parseTipSet(ctx, api, strings.Split(req.Arguments[0], ","))
		}
		if err != nil {
			return err
		}

		if ts == nil {
			return fmt.Errorf("must pass cids for tipset to set as head")
		}

		if err := api.ChainSetHead(ctx, ts.Key()); err != nil {
			return err
		}

		return nil
	},
}

func parseTipSet(ctx context.Context, api *chain.ChainAPI, vals []string) (*block.TipSet, error) {
	var headers []*block.Block
	for _, c := range vals {
		blkc, err := cid.Decode(c)
		if err != nil {
			return nil, err
		}

		bh, err := api.ChainGetBlock(ctx, blkc)
		if err != nil {
			return nil, err
		}

		headers = append(headers, bh)
	}

	return block.NewTipSet(headers...)
}

var storeGasPriceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Estimate gas price",
	},
	Run: func(req *cmds.Request, rem cmds.ResponseEmitter, env cmds.Environment) error {
		nb := []int{1, 2, 3, 5, 10, 20, 50, 100, 300}
		for _, nblocks := range nb {
			addr := builtin.SystemActorAddr // TODO: make real when used in GasEstimateGasPremium

			est, err := env.(*node.Env).MessagePoolAPI.GasEstimateGasPremium(context.TODO(), uint64(nblocks), addr, 10000, block.TipSetKey{})
			if err != nil {
				return err
			}

			_ = rem.Emit(fmt.Sprintf("%d blocks: %s (%s)\n", nblocks, est, types.FIL(est)))
		}

		return nil
	},
}
