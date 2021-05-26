package state

import (
	"context"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// nolint
type contextStore struct {
	ctx context.Context
	cst *cbor.BasicIpldStore
}

// nolint
func (cs *contextStore) Context() context.Context {
	return cs.ctx
}

// nolint
func (cs *contextStore) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	return cs.cst.Get(ctx, c, out)
}

// nolint
func (cs *contextStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return cs.cst.Put(ctx, v)
}
