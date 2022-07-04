package market

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-state-types/builtin/v8/paych"
)

type VoucherInfo struct {
	Voucher   *paych.SignedVoucher
	Proof     []byte // ignored
	Submitted bool
}

type VoucherInfos []*VoucherInfo

func (info *VoucherInfos) Scan(value interface{}) error {
	data, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value must be []byte")
	}
	return json.Unmarshal(data, info)
}

func (info VoucherInfos) Value() (driver.Value, error) {
	return json.Marshal(info)
}

// ChannelInfo keeps track of information about a channel
type ChannelInfo struct {
	// ChannelID is a uuid set at channel creation
	ChannelID string
	// Channel address - may be nil if the channel hasn't been created yet
	Channel *address.Address
	// Control is the address of the local node
	Control address.Address
	// Target is the address of the remote node (on the other end of the channel)
	Target address.Address
	// Direction indicates if the channel is inbound (Control is the "to" address)
	// or outbound (Control is the "from" address)
	Direction uint64
	// Vouchers is a list of all vouchers sent on the channel
	Vouchers []*VoucherInfo
	// NextLane is the number of the next lane that should be used when the
	// client requests a new lane (eg to create a voucher for a new deal)
	NextLane uint64
	// Amount added to the channel.
	// Note: This amount is only used by GetPaych to keep track of how much
	// has locally been added to the channel. It should reflect the channel's
	// Balance on chain as long as all operations occur on the same datastore.
	Amount big.Int
	// PendingAmount is the amount that we're awaiting confirmation of
	PendingAmount big.Int
	// CreateMsg is the CID of a pending create message (while waiting for confirmation)
	CreateMsg *cid.Cid
	// AddFundsMsg is the CID of a pending add funds message (while waiting for confirmation)
	AddFundsMsg *cid.Cid
	// Settling indicates whether the channel has entered into the settling state
	Settling bool
}

func (ci *ChannelInfo) From() address.Address {
	if ci.Direction == DirOutbound {
		return ci.Control
	}
	return ci.Target
}

func (ci *ChannelInfo) To() address.Address {
	if ci.Direction == DirOutbound {
		return ci.Target
	}
	return ci.Control
}

// infoForVoucher gets the VoucherInfo for the given voucher.
// returns nil if the channel doesn't have the voucher.
func (ci *ChannelInfo) InfoForVoucher(sv *paych.SignedVoucher) (*VoucherInfo, error) {
	for _, v := range ci.Vouchers {
		eq, err := cborutil.Equals(sv, v.Voucher)
		if err != nil {
			return nil, err
		}
		if eq {
			return v, nil
		}
	}
	return nil, nil
}

func (ci *ChannelInfo) HasVoucher(sv *paych.SignedVoucher) (bool, error) {
	vi, err := ci.InfoForVoucher(sv)
	return vi != nil, err
}

// markVoucherSubmitted marks the voucher, and any vouchers of lower nonce
// in the same lane, as being submitted.
// Note: This method doesn't write anything to the store.
func (ci *ChannelInfo) MarkVoucherSubmitted(sv *paych.SignedVoucher) error {
	vi, err := ci.InfoForVoucher(sv)
	if err != nil {
		return err
	}
	if vi == nil {
		return fmt.Errorf("cannot submit voucher that has not been added to channel")
	}

	// Mark the voucher as submitted
	vi.Submitted = true

	// Mark lower-nonce vouchers in the same lane as submitted (lower-nonce
	// vouchers are superseded by the submitted voucher)
	for _, vi := range ci.Vouchers {
		if vi.Voucher.Lane == sv.Lane && vi.Voucher.Nonce < sv.Nonce {
			vi.Submitted = true
		}
	}

	return nil
}

// wasVoucherSubmitted returns true if the voucher has been submitted
func (ci *ChannelInfo) WasVoucherSubmitted(sv *paych.SignedVoucher) (bool, error) {
	vi, err := ci.InfoForVoucher(sv)
	if err != nil {
		return false, err
	}
	if vi == nil {
		return false, fmt.Errorf("cannot submit voucher that has not been added to channel")
	}
	return vi.Submitted, nil
}

// MsgInfo stores information about a create channel / add funds message
// that has been sent
type MsgInfo struct {
	// ChannelID links the message to a channel
	ChannelID string
	// MsgCid is the CID of the message
	MsgCid cid.Cid
	// Received indicates whether a response has been received
	Received bool
	// Err is the error received in the response
	Err string
}

const (
	DirInbound  = 1
	DirOutbound = 2
)

var ErrChannelNotFound = fmt.Errorf("channel not found")
