// Package commands implements the command to print the blockchain.
package commands

import (
	"context"

	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/types"
)

type bestBlockGetter func() *types.Block

// func sends either an error or a *types.Block
type blockHistoryGetter func(ctx context.Context) <-chan interface{}

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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return runChainHead(GetNode(env).ChainMgr.GetBestBlock, re.Emit)
	},
	Type: cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		return runChainLs(req.Context, GetNode(env).ChainMgr.BlockHistory, re.Emit)
	},
	Type: types.Block{},
}

func runChainLs(ctx context.Context, getter blockHistoryGetter, emitter valueEmitter) error {
	for raw := range getter(ctx) {
		switch v := raw.(type) {
		case error:
			return v
		case *types.Block:
			emitter(v) // nolint: errcheck
		default:
			return errors.New("unexpected type")
		}
	}

	return nil
}

func runChainHead(getBestBlock bestBlockGetter, emit valueEmitter) error {
	blk := getBestBlock()
	if blk == nil {
		return errors.New("best block not found")
	}

	emit(cmds.Single{Value: blk.Cid()}) // nolint: errcheck

	return nil
}
