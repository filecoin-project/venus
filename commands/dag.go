// Package commands implements the command to print the blockchain.
package commands

import (
	"context"
	"time"

	path "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/path"
	resolver "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/path/resolver"
	cmds "gx/ipfs/QmUf5GFfV2Be3UtSAPKDVkoRd1TwEBTmx9TSSCFGGjNgdQ/go-ipfs-cmds"
	"gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	cmdkit "gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"

	dag "gx/ipfs/QmNUCLv5fmUBuAcwbkt58NQvMcJgd5FPCYV2yNCXq4Wnd6/go-ipfs/merkledag"
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
		n := GetNode(env)

		p, err := path.ParsePath(req.Arguments[0])
		if err != nil {
			return err
		}

		dserv := dag.NewDAGService(n.Blockservice)
		resolver := resolver.NewBasicResolver(dserv)
		obj, rem, err := resolver.ResolveToLastNode(req.Context, p)
		if err != nil {
			return err
		}

		var out interface{} = obj
		if len(rem) > 0 {
			final, _, err := obj.Resolve(rem)
			if err != nil {
				return err
			}
			out = final
		}

		re.Emit(out) // nolint: errcheck
		return nil
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
