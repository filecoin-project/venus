// Package commands implements the command to print the blockchain.
package commands

import (
	"context"
	"time"

	cmds "gx/ipfs/QmVTmXZC2yE38SDKRihn96LXX6KwBWgzAg8aCDZaMirCHm/go-ipfs-cmds"
	"gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
	path "gx/ipfs/Qmet61sdJdo6ZHDc59ZbffN5ED79K5kqfqkRjBx7ncaDg4/go-path"
	resolver "gx/ipfs/Qmet61sdJdo6ZHDc59ZbffN5ED79K5kqfqkRjBx7ncaDg4/go-path/resolver"

	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"
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
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		n := GetNode(env)

		p, err := path.ParsePath(req.Arguments[0])
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		dserv := dag.NewDAGService(n.Blockservice)
		resolver := resolver.NewBasicResolver(dserv)
		objc, rem, err := resolver.ResolveToLastNode(req.Context, p)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		obj, err := dserv.Get(req.Context, objc)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		var out interface{} = obj
		if len(rem) > 0 {
			final, _, err := obj.Resolve(rem)
			if err != nil {
				re.SetError(err, cmdkit.ErrNormal)
				return
			}
			out = final
		}

		re.Emit(out) // nolint: errcheck
	},
}

// runDagGetByCid is used in dag_test.go
func runDagGetByCid(ctx context.Context, get ipldNodeGetter, emit valueEmitter, cid *cid.Cid) error { // nolint: deadcode
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	ipldnode, err := get(ctx, cid)
	if err != nil {
		return err
	}

	emit(cmds.Single{Value: ipldnode}) // nolint: errcheck

	return nil
}
