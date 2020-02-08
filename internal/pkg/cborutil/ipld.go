package cborutil

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

type ReadOnlyIpldStore struct {
	cbor.IpldStore
}

func (s *ReadOnlyIpldStore) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	return cid.Undef, errors.New("readonly store")
}
