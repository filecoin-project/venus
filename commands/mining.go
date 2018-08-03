package commands

import (
	"context"
	"fmt"
	"io"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/state"
)

var miningCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage mining operations",
	},
	Subcommands: map[string]*cmds.Command{
		"once":  miningOnceCmd,
		"start": miningStartCmd,
		"stop":  miningStopCmd,
	},
}

var miningOnceCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)
		ts := fcn.ChainMgr.GetHeaviestTipSet()

		if fcn.RewardAddress().Empty() {
			re.SetError("filecoin node requires a reward address to be set before mining", cmdkit.ErrNormal)
			return
		}

		blockGenerator := mining.NewBlockGenerator(fcn.MsgPool, func(ctx context.Context, ts core.TipSet) (state.Tree, error) {
			return fcn.ChainMgr.State(ctx, ts.ToSlice())
		}, fcn.ChainMgr.Weight, core.ApplyMessages, fcn.ChainMgr.PwrTableView)
		// TODO(EC): Need to read best tipsets from storage and pass in. See also Node::StartMining().
		res := mining.MineOnce(req.Context, mining.NewWorker(blockGenerator), ts, fcn.RewardAddress())
		if res.Err != nil {
			re.SetError(res.Err, cmdkit.ErrNormal)
			return
		}

		if err := fcn.AddNewBlock(req.Context, res.NewBlock); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit(res.NewBlock.Cid()) // nolint: errcheck
	},
	Type: cid.Cid{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, c *cid.Cid) error {
			fmt.Fprintln(w, c) // nolint: errcheck
			return nil
		}),
	},
}

var miningStartCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		if err := GetNode(env).StartMining(); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}
		re.Emit("Started mining") // nolint: errcheck
	},
	Type:     "",
	Encoders: stringEncoderMap,
}

var miningStopCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		GetNode(env).StopMining()
		re.Emit("Stopped mining") // nolint: errcheck
	},
	Encoders: stringEncoderMap,
}

var stringEncoderMap = cmds.EncoderMap{
	cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, t string) error {
		fmt.Fprintln(w, t) // nolint: errcheck
		return nil
	}),
}
