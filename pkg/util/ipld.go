package util

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

// ReadOnlyIpldStore is a store that rejects writes.
type ReadOnlyIpldStore struct {
	cbor.IpldStore
}

// Put returns an error.
func (s *ReadOnlyIpldStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return cid.Undef, errors.New("readonly store")
}
