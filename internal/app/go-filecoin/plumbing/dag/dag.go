package dag

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	chunk "github.com/ipfs/go-ipfs-chunker"
	format "github.com/ipfs/go-ipld-format"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-path"
	"github.com/ipfs/go-path/resolver"
	"github.com/ipfs/go-unixfs"
	imp "github.com/ipfs/go-unixfs/importer"
	uio "github.com/ipfs/go-unixfs/io"
	"github.com/pkg/errors"
)

// DAG is a service for accessing the merkledag
type DAG struct {
	dserv format.DAGService // Provides access to state tree.
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

// Cat returns an iostream with a piece of data stored on the merkeldag with
// the given cid.
//
// TODO: this goes back to 'how is data stored and referenced'
// For now, lets just do things the ipfs way.
// https://github.com/filecoin-project/specs/issues/136
func (dag *DAG) Cat(ctx context.Context, c cid.Cid) (uio.DagReader, error) {
	data, err := dag.dserv.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return uio.NewDagReader(ctx, data, dag.dserv)
}

// ImportData adds data from an io stream to the merkledag and returns the Cid
// of the given data
func (dag *DAG) ImportData(ctx context.Context, data io.Reader) (ipld.Node, error) {
	bufds := ipld.NewBufferedDAG(ctx, dag.dserv)

	spl := chunk.DefaultSplitter(data)

	nd, err := imp.BuildDagFromReader(bufds, spl)
	if err != nil {
		return nil, err
	}
	return nd, bufds.Commit()
}

// RecursiveGet will walk the dag in order (depth first) starting at the given root `c`.
func (dag *DAG) RecursiveGet(ctx context.Context, c cid.Cid) ([]ipld.Node, error) {
	collector := dagCollector{
		dagserv: dag.dserv,
	}
	return collector.collectState(ctx, c)
}

//
// Helpers for recursive dag get.
//

type dagCollector struct {
	dagserv format.DAGService
	state   []format.Node
}

// collectState recursively walks the state tree starting with `stateRoot` and returns it as a slice of IPLD nodes.
// Calling this method does not have any side effects.
func (dc *dagCollector) collectState(ctx context.Context, stateRoot cid.Cid) ([]format.Node, error) {
	dagNd, err := dc.dagserv.Get(ctx, stateRoot)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load stateroot from dagservice %s", stateRoot)
	}
	dc.addState(dagNd)
	seen := cid.NewSet()
	for _, l := range dagNd.Links() {
		if err := dag.Walk(ctx, dc.getLinks, l.Cid, seen.Visit); err != nil {
			return nil, errors.Wrapf(err, "dag service failed walking stateroot %s", stateRoot)
		}
	}
	return dc.state, nil

}

func (dc *dagCollector) getLinks(ctx context.Context, c cid.Cid) ([]*format.Link, error) {
	nd, err := dc.dagserv.Get(ctx, c)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load link from dagservice %s", c)
	}
	dc.addState(nd)
	return nd.Links(), nil

}

func (dc *dagCollector) addState(nd format.Node) {
	dc.state = append(dc.state, nd)
}
