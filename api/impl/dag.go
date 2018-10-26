package impl

import (
	"context"

	path "gx/ipfs/QmNmKx3KvS3gtSws9idLwyNFV4uYhGmxX8NWwemdcdH4G5/go-path"
	resolver "gx/ipfs/QmNmKx3KvS3gtSws9idLwyNFV4uYhGmxX8NWwemdcdH4G5/go-path/resolver"
	dag "gx/ipfs/QmeLG6jF1xvEmHca5Vy4q4EdQWp8Xq9S6EPyZrN9wvSRLC/go-merkledag"
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
