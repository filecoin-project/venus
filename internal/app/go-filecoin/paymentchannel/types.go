package paymentchannel

import (
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

//go:generate cbor-gen-for ChannelInfo VoucherInfo

// The key for the store is the payment channel's "robust" or "unique" address

// ChannelInfo is the primary payment channel record
// IDAddr: internal ID address for payment channel actor
// UniqueAddr: aka RobustAddr, used externally to refer to payment channel
// Duplicated in data due to need to check for existing payment channel since you can't
// iterate over key/value pairs in statestore at present
type ChannelInfo struct {
	UniqueAddr, IDAddr, From, To address.Address
	LastLane                     uint64
	Vouchers                     []*VoucherInfo // All vouchers submitted for this channel
}

// ChannelInfoUndef is an empty ChannelInfo
var ChannelInfoUndef = &ChannelInfo{}

// Equal is a comparator for two ChannelInfos
func (ci *ChannelInfo) Equal(ci2 *ChannelInfo) bool {
	if ci.UniqueAddr == ci2.UniqueAddr &&
		ci.To == ci2.To &&
		ci.From == ci2.From &&
		ci.LastLane == ci2.LastLane &&
		reflect.DeepEqual(ci.Vouchers, ci2.Vouchers) {
		return true
	}
	return false
}

// VoucherInfo is a record of a voucher submitted for a payment channel
type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}
