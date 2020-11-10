package vmcontext

import (
	"context"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/internal/pkg/specactors/adt"
	"github.com/filecoin-project/venus/internal/pkg/vm/storage"
)

// Dragons: see if we can reuse the `adt.AsStore` Method To construct this instead of re-writing it
type contextStore struct {
	context context.Context
	store   *storage.VMStorage
}

// implement adt.Store

var _ adt.Store = (*contextStore)(nil)

func (a *contextStore) Context() context.Context {
	return a.context
}

// (implement cbor.IpldStore, part of adt.Store)

func (a *contextStore) Get(ctx context.Context, id cid.Cid, obj interface{}) error {
	err := a.store.Get(ctx, id, obj)
	return err
}

func (a *contextStore) Put(ctx context.Context, obj interface{}) (cid.Cid, error) {
	id, err := a.store.Put(ctx, obj)
	return id, err
}
