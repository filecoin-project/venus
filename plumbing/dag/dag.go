package dag

import (
	"context"

	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmVUojkFtcsrVBa8kYZiM6LhxpYXaKDTxE4aF1NFj4RfBv/go-path"
	"gx/ipfs/QmVUojkFtcsrVBa8kYZiM6LhxpYXaKDTxE4aF1NFj4RfBv/go-path/resolver"
)

// DAG is the
type DAG struct {
	dserv ipld.DAGService
}

// NewDAG creates a DAG with a given DAGService
func NewDAG(dserv ipld.DAGService) *DAG {
	return &DAG{
		dserv: dserv,
	}
}

// Get returns the associated DAG node for the passed in CID.
func (dag *DAG) Get(ctx context.Context, ref string) (interface{}, error) {
	parsedRef, err := path.ParsePath(ref)
	if err != nil {
		return nil, err
	}

	resolver := resolver.NewBasicResolver(dag.dserv)

	objc, rem, err := resolver.ResolveToLastNode(ctx, parsedRef)
	if err != nil {
		return nil, err
	}

	obj, err := dag.dserv.Get(ctx, objc)
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
