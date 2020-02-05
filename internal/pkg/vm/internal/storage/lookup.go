package storage

import (
	"context"
	"github.com/ipfs/go-cid"
)

// ValueCallbackFunc is called when iterating nodes with a HAMT lookup in order
// with a key and a decoded value
type ValueCallbackFunc func(k string, v interface{}) error

// Lookup defines an internal interface for actor storage.
type Lookup interface {
	Find(ctx context.Context, k string, out interface{}) error
	Set(ctx context.Context, k string, v interface{}) error
	Commit(ctx context.Context) (cid.Cid, error)
	Delete(ctx context.Context, k string) error
	IsEmpty() bool
	ForEachValue(ctx context.Context, valueType interface{}, callback ValueCallbackFunc) error
}
