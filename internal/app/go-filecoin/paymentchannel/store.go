package paymentchannel

import (
	"bytes"
	"errors"
	"sync"

	"github.com/filecoin-project/go-address"
	cborrpc "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
)

//go:generate cbor-gen-for ChannelInfo VoucherInfo

var ErrChannelNotTracked = errors.New("channel not tracked")

type Store struct {
	lks []sync.Mutex // TODO: this can be split per paych (from lotus)

	ds datastore.Batching
}

func NewStore(ds datastore.Batching) *Store {
	ds = namespace.Wrap(ds, datastore.NewKey("/paych/"))
	return &Store{
		ds: ds,
	}
}

type ChannelInfo struct {
	Owner    address.Address // Payout (From) address for this channel, has ability to sign and send funds
	State    *paych.State
	Vouchers []*VoucherInfo // All vouchers submitted for this channel
}

type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}

func dskeyForChannel(payChAddr address.Address) datastore.Key {
	return datastore.NewKey(payChAddr.String())
}

func (ps *Store) putChannelInfo(payChAddr address.Address, ci *ChannelInfo) error {
	k := dskeyForChannel(payChAddr)

	b, err := cborrpc.Dump(ci)
	if err != nil {
		return err
	}

	return ps.ds.Put(k, b)
}

// getChannelInfo retrieves a ChannelInfo record from the store via the payment channel actor address.
// returns error if the lookup fails.
func (ps *Store) getChannelInfo(payChAddr address.Address) (*ChannelInfo, error) {
	k := dskeyForChannel(payChAddr)

	b, err := ps.ds.Get(k)
	if err == datastore.ErrNotFound {
		return nil, ErrChannelNotTracked
	}
	if err != nil {
		return nil, err
	}

	var ci ChannelInfo
	if err := ci.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, err
	}

	return &ci, nil
}
