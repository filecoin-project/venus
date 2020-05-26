package vmcontext

import (
	"context"
	"fmt"
	"reflect"

	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
)

type vmStorage interface {
	Get(ctx context.Context, cid cid.Cid, obj interface{}) (int, error)
	Put(ctx context.Context, obj interface{}) (cid.Cid, int, error)
}

// ActorStorage hides the storage methods from the actors and turns the errors into runtime panics.
type ActorStorage struct {
	context   context.Context
	inner     vmStorage
	pricelist gascost.Pricelist
	gasTank   *GasTracker
}

func NewActorStorage(ctx context.Context, inner vmStorage, gasTank *GasTracker, pricelist gascost.Pricelist) *ActorStorage {
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

var _ specsruntime.Store = (*ActorStorage)(nil)

// Serialization technically belongs in the actor code, rather than inside the VM.
// The true VM storage interface is in terms of raw bytes and, when we have user-defined,
// serialization code will be directly in those contracts.
// Our present runtime interface is at a slightly higher level for convenience, but the exit code here is the
// actor, rather than system-level, error code.
const serializationErr = exitcode.ErrSerialization

func (s *ActorStorage) Put(obj specsruntime.CBORMarshaler) cid.Cid {
	cid, ln, err := s.inner.Put(s.context, obj)
	if err != nil {
		msg := fmt.Sprintf("failed to put object %s in store: %s", reflect.TypeOf(obj), err)
		if _, ok := err.(storage.SerializationError); ok {
			runtime.Abortf(serializationErr, msg)
		} else {
			panic(msg)
		}
	}
	s.gasTank.Charge(s.pricelist.OnIpldPut(ln), "storage put %s %d bytes into %v", cid, ln, obj)
	return cid
}

func (s *ActorStorage) Get(cid cid.Cid, obj specsruntime.CBORUnmarshaler) bool {
	ln, err := s.inner.Get(s.context, cid, obj)
	if err == storage.ErrNotFound {
		return false
	}
	if err != nil {
		msg := fmt.Sprintf("failed to get object %s %s from store: %s", reflect.TypeOf(obj), cid, err)
		if _, ok := err.(storage.SerializationError); ok {
			runtime.Abortf(serializationErr, msg)
		} else {
			panic(msg)
		}
	}
	s.gasTank.Charge(s.pricelist.OnIpldGet(ln), "storage get %s %d bytes into %v", cid, ln, obj)
	return true
}
