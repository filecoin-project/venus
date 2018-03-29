// Package commands implements the command to print the blockchain.
package commands

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

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
		Tagline: "get the best block CID",
	},
	Run:  chainHeadRun,
	Type: cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run:  chainLsRun,
	Type: types.Block{},
}

func chainLsRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	n := GetNode(env)

	blk := n.ChainMgr.GetBestBlock()
	re.Emit(blk) // nolint: errcheck

	for blk.Parent != nil {
		var next types.Block
		if err := n.CborStore.Get(req.Context, blk.Parent, &next); err != nil {
			return err
		}
		re.Emit(&next) // nolint: errcheck
		blk = &next
	}

	return nil
}

func chainHeadRun(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
	n := GetNode(env)

	blk := n.ChainMgr.GetBestBlock()
	if blk == nil {
		return errors.New("best block not found")
	}

	cmds.EmitOnce(re, blk.Cid()) // nolint: errcheck

	return nil
}
