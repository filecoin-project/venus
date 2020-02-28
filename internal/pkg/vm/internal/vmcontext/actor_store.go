package vmcontext

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/gascost"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/ipfs/go-cid"
)

// actorStorage hides the storage methods from the actors and turns the errors into runtime panics.
type actorStorage struct {
	context   context.Context
	inner     *storage.VMStorage
	pricelist gascost.Pricelist
	gasTank   *GasTracker
}

//
// implement runtime.Store for actorStorage
//

var _ specsruntime.Store = (*actorStorage)(nil)

func (s actorStorage) Put(obj specsruntime.CBORMarshaler) cid.Cid {
	cid, ln, err := s.inner.Put(s.context, obj)
	if err != nil {
		panic(fmt.Errorf("could not put object in store. %s", err))
	}
	s.gasTank.Charge(s.pricelist.OnIpldPut(ln))
	return cid
}

func (s actorStorage) Get(cid cid.Cid, obj specsruntime.CBORUnmarshaler) bool {
	ln, err := s.inner.Get(s.context, cid, obj)
	if err == storage.ErrNotFound {
		return false
	}
	if err != nil {
		panic(fmt.Errorf("could not get obj from store. %s", err))
	}
	s.gasTank.Charge(s.pricelist.OnIpldGet(ln))
	return true
}
