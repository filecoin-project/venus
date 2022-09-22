package paychmgr

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	dsq "github.com/ipfs/go-datastore/query"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v8/paych"

	"github.com/filecoin-project/venus/pkg/repo"
	pchTypes "github.com/filecoin-project/venus/venus-shared/types/market"
)

var ErrChannelNotTracked = errors.New("channel not tracked")

type Store struct {
	ds datastore.Datastore
}

// for test
func NewStore(ds repo.Datastore) *Store {
	dsTmp := namespace.Wrap(ds, datastore.NewKey("/paych/"))
	return &Store{
		ds: dsTmp,
	}
}

const (
	dsKeyChannelInfo = "ChannelInfo"
	dsKeyMsgCid      = "MsgCid"
)

// TrackChannel stores a channel, returning an error if the channel was already
// being tracked
func (ps *Store) TrackChannel(ctx context.Context, ci *pchTypes.ChannelInfo) (*pchTypes.ChannelInfo, error) {
	_, err := ps.ByAddress(ctx, *ci.Channel)
	switch err {
	default:
		return nil, err
	case nil:
		return nil, fmt.Errorf("already tracking channel: %s", ci.Channel)
	case ErrChannelNotTracked:
		err = ps.putChannelInfo(ctx, ci)
		if err != nil {
			return nil, err
		}

		return ps.ByAddress(ctx, *ci.Channel)
	}
}

// ListChannels returns the addresses of all channels that have been created
func (ps *Store) ListChannels(ctx context.Context) ([]address.Address, error) {
	cis, err := ps.findChans(ctx, func(ci *pchTypes.ChannelInfo) bool {
		return ci.Channel != nil
	}, 0)
	if err != nil {
		return nil, err
	}

	addrs := make([]address.Address, 0, len(cis))
	for _, ci := range cis {
		addrs = append(addrs, *ci.Channel)
	}

	return addrs, nil
}

// findChan finds a single channel using the given filter.
// If there isn't a channel that matches the filter, returns ErrChannelNotTracked
func (ps *Store) findChan(ctx context.Context, filter func(ci *pchTypes.ChannelInfo) bool) (*pchTypes.ChannelInfo, error) {
	cis, err := ps.findChans(ctx, filter, 1)
	if err != nil {
		return nil, err
	}

	if len(cis) == 0 {
		return nil, ErrChannelNotTracked
	}

	return &cis[0], err
}

// findChans loops over all channels, only including those that pass the filter.
// max is the maximum number of channels to return. Set to zero to return unlimited channels.
func (ps *Store) findChans(ctx context.Context, filter func(*pchTypes.ChannelInfo) bool, max int) ([]pchTypes.ChannelInfo, error) {
	res, err := ps.ds.Query(ctx, dsq.Query{Prefix: dsKeyChannelInfo})
	if err != nil {
		return nil, err
	}
	defer res.Close() //nolint:errcheck

	var stored pchTypes.ChannelInfo
	var matches []pchTypes.ChannelInfo

	for {
		res, ok := res.NextSync()
		if !ok {
			break
		}

		if res.Error != nil {
			return nil, err
		}

		ci, err := unmarshallChannelInfo(&stored, res.Value)
		if err != nil {
			return nil, err
		}

		if !filter(ci) {
			continue
		}

		matches = append(matches, *ci)

		// If we've reached the maximum number of matches, return.
		// Note that if max is zero we return an unlimited number of matches
		// because len(matches) will always be at least 1.
		if len(matches) == max {
			return matches, nil
		}
	}

	return matches, nil
}

// AllocateLane allocates a new lane for the given channel
func (ps *Store) AllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	ci, err := ps.ByAddress(ctx, ch)
	if err != nil {
		return 0, err
	}

	out := ci.NextLane
	ci.NextLane++

	return out, ps.putChannelInfo(ctx, ci)
}

// VouchersForPaych gets the vouchers for the given channel
func (ps *Store) VouchersForPaych(ctx context.Context, ch address.Address) ([]*pchTypes.VoucherInfo, error) {
	ci, err := ps.ByAddress(ctx, ch)
	if err != nil {
		return nil, err
	}

	return ci.Vouchers, nil
}

func (ps *Store) MarkVoucherSubmitted(ctx context.Context, ci *pchTypes.ChannelInfo, sv *paych.SignedVoucher) error {
	err := ci.MarkVoucherSubmitted(sv)
	if err != nil {
		return err
	}
	return ps.putChannelInfo(ctx, ci)
}

// ByAddress gets the channel that matches the given address
func (ps *Store) ByAddress(ctx context.Context, addr address.Address) (*pchTypes.ChannelInfo, error) {
	return ps.findChan(ctx, func(ci *pchTypes.ChannelInfo) bool {
		return ci.Channel != nil && *ci.Channel == addr
	})
}

// The datastore key used to identify the message
func dskeyForMsg(mcid cid.Cid) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyMsgCid, mcid.String()})
}

// SaveNewMessage is called when a message is sent
func (ps *Store) SaveNewMessage(ctx context.Context, channelID string, mcid cid.Cid) error {
	k := dskeyForMsg(mcid)

	b, err := cborutil.Dump(&pchTypes.MsgInfo{ChannelID: channelID, MsgCid: mcid})
	if err != nil {
		return err
	}

	return ps.ds.Put(ctx, k, b)
}

// SaveMessageResult is called when the result of a message is received
func (ps *Store) SaveMessageResult(ctx context.Context, mcid cid.Cid, msgErr error) error {
	minfo, err := ps.GetMessage(ctx, mcid)
	if err != nil {
		return err
	}

	k := dskeyForMsg(mcid)
	minfo.Received = true
	if msgErr != nil {
		minfo.Err = msgErr.Error()
	}

	b, err := cborutil.Dump(minfo)
	if err != nil {
		return err
	}

	return ps.ds.Put(ctx, k, b)
}

// ByMessageCid gets the channel associated with a message
func (ps *Store) ByMessageCid(ctx context.Context, mcid cid.Cid) (*pchTypes.ChannelInfo, error) {
	minfo, err := ps.GetMessage(ctx, mcid)
	if err != nil {
		return nil, err
	}

	ci, err := ps.findChan(ctx, func(ci *pchTypes.ChannelInfo) bool {
		return ci.ChannelID == minfo.ChannelID
	})
	if err != nil {
		return nil, err
	}

	return ci, err
}

// GetMessage gets the message info for a given message CID
func (ps *Store) GetMessage(ctx context.Context, mcid cid.Cid) (*pchTypes.MsgInfo, error) {
	k := dskeyForMsg(mcid)

	val, err := ps.ds.Get(ctx, k)
	if err != nil {
		return nil, err
	}

	var minfo pchTypes.MsgInfo
	if err := minfo.UnmarshalCBOR(bytes.NewReader(val)); err != nil {
		return nil, err
	}

	return &minfo, nil
}

// OutboundActiveByFromTo looks for outbound channels that have not been
// settled, with the given from / to addresses
func (ps *Store) OutboundActiveByFromTo(ctx context.Context, sma managerAPI, from address.Address, to address.Address) (*pchTypes.ChannelInfo, error) {
	return ps.findChan(ctx, func(ci *pchTypes.ChannelInfo) bool {
		if ci.Direction != pchTypes.DirOutbound {
			return false
		}
		if ci.Settling {
			return false
		}
		if ci.Channel != nil {
			_, st, err := sma.GetPaychState(ctx, *ci.Channel, nil)
			if err != nil {
				return false
			}
			sat, err := st.SettlingAt()
			if err != nil {
				return false
			}
			if sat != 0 {
				return false
			}
		}
		return ci.Control == from && ci.Target == to
	})
}

// WithPendingAddFunds is used on startup to find channels for which a
// create channel or add funds message has been sent, but lotus shut down
// before the response was received.
func (ps *Store) WithPendingAddFunds(ctx context.Context) ([]pchTypes.ChannelInfo, error) {
	return ps.findChans(ctx, func(ci *pchTypes.ChannelInfo) bool {
		if ci.Direction != pchTypes.DirOutbound {
			return false
		}
		return ci.CreateMsg != nil || ci.AddFundsMsg != nil
	}, 0)
}

// ByChannelID gets channel info by channel ID
func (ps *Store) ByChannelID(ctx context.Context, channelID string) (*pchTypes.ChannelInfo, error) {
	var stored pchTypes.ChannelInfo

	res, err := ps.ds.Get(ctx, dskeyForChannel(channelID))
	if err != nil {
		if err == datastore.ErrNotFound {
			return nil, ErrChannelNotTracked
		}
		return nil, err
	}

	return unmarshallChannelInfo(&stored, res)
}

// CreateChannel creates an outbound channel for the given from / to
func (ps *Store) CreateChannel(ctx context.Context, from address.Address, to address.Address, createMsgCid cid.Cid, amt, avail fbig.Int) (*pchTypes.ChannelInfo, error) {
	ci := &pchTypes.ChannelInfo{
		Direction:              pchTypes.DirOutbound,
		NextLane:               0,
		Control:                from,
		Target:                 to,
		CreateMsg:              &createMsgCid,
		PendingAmount:          amt,
		PendingAvailableAmount: avail,
	}

	// Save the new channel
	err := ps.putChannelInfo(ctx, ci)
	if err != nil {
		return nil, err
	}

	// Save a reference to the create message
	err = ps.SaveNewMessage(ctx, ci.ChannelID, createMsgCid)
	if err != nil {
		return nil, err
	}

	return ci, err
}

// RemoveChannel removes the channel with the given channel ID
func (ps *Store) RemoveChannel(ctx context.Context, channelID string) error {
	return ps.ds.Delete(ctx, dskeyForChannel(channelID))
}

// The datastore key used to identify the channel info
func dskeyForChannel(channelID string) datastore.Key {
	return datastore.KeyWithNamespaces([]string{dsKeyChannelInfo, channelID})
}

// putChannelInfo stores the channel info in the datastore
func (ps *Store) putChannelInfo(ctx context.Context, ci *pchTypes.ChannelInfo) error {
	if len(ci.ChannelID) == 0 {
		ci.ChannelID = uuid.New().String()
	}
	k := dskeyForChannel(ci.ChannelID)

	b, err := marshallChannelInfo(ci)
	if err != nil {
		return err
	}

	return ps.ds.Put(ctx, k, b)
}

// TODO: This is a hack to get around not being able to CBOR marshall a nil
// address.Address. It's been fixed in address.Address but we need to wait
// for the change to propagate to specs-actors before we can remove this hack.
var emptyAddr address.Address

func init() {
	addr, err := address.NewActorAddress([]byte("empty"))
	if err != nil {
		panic(err)
	}
	emptyAddr = addr
}

func marshallChannelInfo(ci *pchTypes.ChannelInfo) ([]byte, error) {
	// See note above about CBOR marshalling address.Address
	if ci.Channel == nil {
		ci.Channel = &emptyAddr
	}
	return cborutil.Dump(ci)
}

func unmarshallChannelInfo(stored *pchTypes.ChannelInfo, value []byte) (*pchTypes.ChannelInfo, error) {
	if err := stored.UnmarshalCBOR(bytes.NewReader(value)); err != nil {
		return nil, err
	}

	// See note above about CBOR marshalling address.Address
	if stored.Channel != nil && *stored.Channel == emptyAddr {
		stored.Channel = nil
	}

	// backwards compat
	if stored.AvailableAmount.Int == nil {
		stored.AvailableAmount = fbig.NewInt(0)
		stored.PendingAvailableAmount = fbig.NewInt(0)
	}

	return stored, nil
}
