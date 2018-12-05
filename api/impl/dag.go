package impl

import (
	"context"

	path "gx/ipfs/QmQtg7N4XjAk2ZYpBjjv8B6gQprsRekabHBCnF6i46JYKJ/go-path"
	resolver "gx/ipfs/QmQtg7N4XjAk2ZYpBjjv8B6gQprsRekabHBCnF6i46JYKJ/go-path/resolver"
	dag "gx/ipfs/QmdURv6Sbob8TVW2tFFve9vcEWrSUgwPqeqnXyvYhLrkyd/go-merkledag"
)

type nodeDag struct {
	api *nodeAPI
}

func newNodeDag(api *nodeAPI) *nodeDag {
	return &nodeDag{api: api}
}

// Get returns the associated DAG node for the passed in CID.
func (api *nodeDag) Get(ctx context.Context, ref string) (interface{}, error) {
	parsedRef, err := path.ParsePath(ref)
	if err != nil {
		return nil, err
	}

	dserv := dag.NewDAGService(api.api.node.BlockService())
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
