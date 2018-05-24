// Package commands implements the command to print the blockchain.
package commands

import (
	"gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		blk := GetNode(env).ChainMgr.GetBestBlock()
		if blk == nil {
			return errors.New("best block not found")
		}

		re.Emit(cmds.Single{Value: blk.Cid()}) // nolint: errcheck

		return nil
	},
	Type: cid.Cid{},
}

var chainLsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "dump full block chain",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		for raw := range GetNode(env).ChainMgr.BlockHistory(req.Context) {
			switch v := raw.(type) {
			case error:
				return v
			case *types.Block:
				re.Emit(v) // nolint: errcheck
			default:
				return errors.New("unexpected type")
			}
		}

		return nil
	},
	Type: types.Block{},
}
