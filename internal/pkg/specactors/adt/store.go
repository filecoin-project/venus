package adt

import (
	"context"

	cbor "github.com/ipfs/go-ipld-cbor"

	//"github.com/filecoin-project/go-filecoin/internal/pkg/specactors"
	adt0 "github.com/filecoin-project/specs-actors/actors/util/adt"
	//adt2 "github.com/filecoin-project/specs-actors/v2/actors/util/adt"
)

type Store interface {
	Context() context.Context
	cbor.IpldStore
}

func WrapStore(ctx context.Context, store cbor.IpldStore/*, version specactors.Version*/) Store {
	//switch version {
	//case specactors.Version0:
	//	return adt0.WrapStore(ctx, store)
	//case specactors.Version2:
	//	return adt2.WrapStore(ctx, store)
	//}
	return adt0.WrapStore(ctx, store)
}
