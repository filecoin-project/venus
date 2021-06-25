package vmcontext

import (
	"context"
	"fmt"
	"reflect"

	"github.com/filecoin-project/go-state-types/exitcode"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	"github.com/ipfs/go-cid"
	cbornode "github.com/ipfs/go-ipld-cbor"
	xerrors "github.com/pkg/errors"

	"github.com/filecoin-project/go-state-types/cbor"
	"github.com/filecoin-project/venus/pkg/vm/gas"
	"github.com/filecoin-project/venus/pkg/vm/runtime"
)

// ActorStorage hides the storage methods From the actors and turns the errors into runtime panics.
type ActorStorage struct {
	context   context.Context
	inner     cbornode.IpldStore
	pricelist gas.Pricelist
	gasTank   *gas.GasTracker
}

func NewActorStorage(ctx context.Context, inner cbornode.IpldStore, gasTank *gas.GasTracker, pricelist gas.Pricelist) *ActorStorage {
	return &ActorStorage{
		context:   ctx,
		inner:     inner,
		pricelist: pricelist,
		gasTank:   gasTank,
	}
}

//
// implement runtime.Store for ActorStorage
//

var _ rt5.Store = (*ActorStorage)(nil)

// Serialization technically belongs in the actor code, rather than inside the VM.
// The true VM storage interface is in terms of raw bytes and, when we have user-defined,
// serialization code will be directly in those contracts.
// Our present runtime interface is at a slightly higher level for convenience, but the exit code here is the
// actor, rather than system-level, error code.
const serializationErr = exitcode.ErrSerialization

func (s *ActorStorage) StorePut(obj cbor.Marshaler) cid.Cid {
	cid, err := s.inner.Put(s.context, obj)
	if err != nil {
		msg := fmt.Sprintf("failed To put object %s in store: %s", reflect.TypeOf(obj), err)
		if xerrors.As(err, new(cbornode.SerializationError)) {
			runtime.Abortf(serializationErr, msg)
		} else {
			panic(msg)
		}
	}
	return cid
}

type notFoundErr interface {
	IsNotFound() bool
}

func (s *ActorStorage) StoreGet(cid cid.Cid, obj cbor.Unmarshaler) bool {
	//gas charge must check first
	if err := s.inner.Get(s.context, cid, obj); err != nil {
		msg := fmt.Sprintf("failed To get object %s %s From store: %s", reflect.TypeOf(obj), cid, err)
		var nfe notFoundErr
		if xerrors.As(err, &nfe) && nfe.IsNotFound() {
			if xerrors.As(err, new(cbornode.SerializationError)) {
				runtime.Abortf(serializationErr, msg)
			}
			return false
		}
		panic(msg)
	}
	return true
}
