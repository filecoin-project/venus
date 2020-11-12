package vmcontext

import (
	"context"
	"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

var _ cbor.IpldStore = (*GasChargeStorage)(nil)

// ActorStorage hides the storage methods From the actors and turns the errors into runtime panics.
type GasChargeStorage struct {
	context   context.Context
	inner     vmStorage
	pricelist gas.Pricelist
	gasTank   *GasTracker
}

func (s *GasChargeStorage) Get(ctx context.Context, c cid.Cid, out interface{}) error {
	//gas charge must check first
	s.gasTank.Charge(s.pricelist.OnIpldGet(), "storage get %s bytes into %v", c, out)
	_, err := s.inner.GetWithLen(s.context, c, out)
	return err
}

func (s *GasChargeStorage) Put(ctx context.Context, v interface{}) (cid.Cid, error) {
	cid, ln, err := s.inner.PutWithLen(s.context, v)
	//fmt.Println("gas storage put ", cid.String())
	s.gasTank.Charge(s.pricelist.OnIpldPut(ln), "storage put %s %d bytes into %v", cid, ln, v)
	return cid, err
}
