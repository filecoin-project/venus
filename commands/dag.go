// Package commands implements the command to print the blockchain.
package commands

import (
	"context"
	"time"

	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmds "gx/ipfs/QmYMj156vnPY7pYvtkvQiMDAzqWDDHkfiW5bYbMpYoHxhB/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	dag "github.com/ipfs/go-ipfs/merkledag"
)

type ipldNodeGetter func(ctx context.Context, c *cid.Cid) (ipld.Node, error)

type valueEmitter func(value interface{}) error

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
		cid, err := cid.Decode(req.Arguments[0])
		if err != nil {
			return errors.New("could not parse argument as CID")
		}

		return runDagGetByCid(req.Context, dag.NewDAGService(GetNode(env).Blockservice).Get, re.Emit, cid)
	},
}

func runDagGetByCid(ctx context.Context, get ipldNodeGetter, emit valueEmitter, cid *cid.Cid) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	ipldnode, err := get(ctx, cid)
	if err != nil {
		return err
	}

	emit(cmds.Single{Value: ipldnode}) // nolint: errcheck

	return nil
}
