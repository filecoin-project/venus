package paymentchannel

import (
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

//go:generate cbor-gen-for ChannelInfo VoucherInfo

// The key for the store is the payment channel's "robust" or "unique" address

// ChannelInfo is the primary payment channel record
// UniqueAddr: aka RobustAddr, used externally to refer to payment channel
// Duplicated in data due to need to check for existing payment channel since you can't
// iterate over key/value pairs in statestore at present
type ChannelInfo struct {
	UniqueAddr, From, To address.Address
	NextLane             uint64
	Vouchers             []*VoucherInfo // All vouchers submitted for this channel
}

// IsZero returns whether it is a zeroed/blank ChannelInfo
func (ci *ChannelInfo) IsZero() bool {
	return ci.UniqueAddr.Empty() && ci.To.Empty() && ci.From.Empty() &&
		ci.NextLane == 0 && len(ci.Vouchers) == 0
}

// hasVoucher returns true if `voucher` is already in `info`
func (ci *ChannelInfo)HasVoucher(voucher *paych.SignedVoucher) bool {
	for _, v := range ci.Vouchers {
		if reflect.DeepEqual(*v.Voucher, *voucher) {
			return true
		}
	}
	return false
}

// VoucherInfo is a record of a voucher submitted for a payment channel
type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}
