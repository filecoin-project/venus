package dag

import (
	"context"
	"fmt"

	"gx/ipfs/QmNRAuGmvnVw8urHkUZQirhu42VTiZjVWASa2aTznEMmpP/go-merkledag"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmRDWTzVdbHXdtat7tVJ7YC7kRaW7rTZTEF79yykcLYa49/go-unixfs"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmVUojkFtcsrVBa8kYZiM6LhxpYXaKDTxE4aF1NFj4RfBv/go-path"
	"gx/ipfs/QmVUojkFtcsrVBa8kYZiM6LhxpYXaKDTxE4aF1NFj4RfBv/go-path/resolver"
)

// DAG is a service for accessing the merkledag
type DAG struct {
	dserv ipld.DAGService
}

// NewDAG creates a DAG with a given DAGService
func NewDAG(dserv ipld.DAGService) *DAG {
	return &DAG{
		dserv: dserv,
	}
}

// GetNode returns the associated DAG node for the passed in CID.
func (dag *DAG) GetNode(ctx context.Context, ref string) (interface{}, error) {
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

// GetFileSize returns the file size for a given Cid
func (dag *DAG) GetFileSize(ctx context.Context, c cid.Cid) (uint64, error) {
	fnode, err := dag.dserv.Get(ctx, c)
	if err != nil {
		return 0, err
	}
	switch n := fnode.(type) {
	case *merkledag.ProtoNode:
		return unixfs.DataSize(n.Data())
	case *merkledag.RawNode:
		return n.Size()
	default:
		return 0, fmt.Errorf("unrecognized node type: %T", fnode)
	}
}
