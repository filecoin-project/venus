package commands

import (
	"context"
	"fmt"
	"io"
	"math/big"

	cmds "gx/ipfs/QmRv6ddf7gkiEgBs1LADv3vC1mkVGPZEfByoiiVybjE9Mc/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/types"
)

var minerCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage mining operations",
	},
	Subcommands: map[string]*cmds.Command{
		"gen-block":    minerGenBlockCmd,
		"start-mining": minerStartMiningCmd,
		"stop-mining":  minerStopMiningCmd,
	},
}

var minerGenBlockCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		fcn := GetNode(env)

		cur := fcn.ChainMgr.GetBestBlock()

		addrs := fcn.Wallet.GetAddresses()
		if len(addrs) == 0 {
			re.SetError("no addresses in wallet to mine to", cmdkit.ErrNormal)
			return
		}
		myaddr := addrs[0]

		reward := types.NewMessage(types.Address("filecoin"), myaddr, big.NewInt(1000), "", nil)
		if _, err := fcn.MsgPool.Add(reward); err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		blockGenerator := mining.NewBlockGenerator(fcn.MsgPool, func(ctx context.Context, cid *cid.Cid) (types.StateTree, error) {
			return types.LoadStateTree(ctx, fcn.CborStore, cid)
		}, core.ProcessBlock)
		res := mining.MineOnce(req.Context, mining.NewWorker(blockGenerator), cur)
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
			fmt.Fprintln(w, c)
			return nil
		}),
	},
}

var minerStartMiningCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		GetNode(env).StartMining(context.Background())
		re.Emit("Started mining\n") // nolint: errcheck
	},
}

var minerStopMiningCmd = &cmds.Command{
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		GetNode(env).StopMining()
		re.Emit("Stopped mining\n") // nolint: errcheck
	},
}
