package paymentchannel

import (
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
	LastLane             uint64
	Vouchers             []*VoucherInfo // All vouchers submitted for this channel
}

// Blank returns whether it is a blank ChannelInfo, rather than
// a comparison to a declared 'undef' value for efficiency
func (ci *ChannelInfo) Blank() bool {
	return ci.UniqueAddr.Empty() && ci.To.Empty() && ci.From.Empty() &&
		ci.LastLane == 0 && len(ci.Vouchers) == 0
}

// VoucherInfo is a record of a voucher submitted for a payment channel
type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}
