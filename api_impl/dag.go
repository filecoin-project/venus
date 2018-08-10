package api_impl

import (
	"context"

	dag "gx/ipfs/QmeCaeBmCCEJrZahwXY4G2G8zRaNBWskrfKWoQ6Xv6c1DR/go-merkledag"
	path "gx/ipfs/Qmet61sdJdo6ZHDc59ZbffN5ED79K5kqfqkRjBx7ncaDg4/go-path"
	resolver "gx/ipfs/Qmet61sdJdo6ZHDc59ZbffN5ED79K5kqfqkRjBx7ncaDg4/go-path/resolver"
)

type NodeDag struct {
	api *NodeAPI
}

func NewNodeDag(api *NodeAPI) *NodeDag {
	return &NodeDag{api: api}
}

// Get returns the associated DAG node for the passed in CID.
func (api *NodeDag) Get(ctx context.Context, ref string) (interface{}, error) {
	parsedRef, err := path.ParsePath(ref)
	if err != nil {
		return nil, err
	}

	dserv := dag.NewDAGService(api.api.node.Blockservice)
	resolver := resolver.NewBasicResolver(dserv)

	objc, rem, err := resolver.ResolveToLastNode(ctx, parsedRef)
	if err != nil {
		return nil, err
	}

	obj, err := dserv.Get(ctx, objc)
	if err != nil {
		return nil, err
	}

	var out interface{} = obj
	if len(rem) > 0 {
		final, _, err := obj.Resolve(rem)
		if err != nil {
			return nil, err
		}
		out = final
	}

	return out, nil
}
