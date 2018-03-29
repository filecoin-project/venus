// Package commands implements the command to print the blockchain.
package commands

import (
	"context"
	"time"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	dag "github.com/ipfs/go-ipfs/merkledag"
)

var dagCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with IPLD DAG objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"get": dagGetCmd,
	},
}

var dagGetCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Get a DAG node by its CID",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("ref", true, false, "CID of object to get"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		n := GetNode(env)

		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return errors.New("could not parse argument as CID")
		}

		ctx, cancel := context.WithTimeout(req.Context, time.Second*10)
		defer cancel()

		ipldnode, err := dag.NewDAGService(n.Blockservice).Get(ctx, cid)
		if err != nil {
			return err
		}

		re.Emit(ipldnode) // nolint: errcheck

		return nil
	},
}
