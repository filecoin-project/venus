package paymentchannel

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

// ChannelInfo is the primary payment channel record
type ChannelInfo struct {
	Owner    address.Address // Payout (From) address for this channel, has ability to sign and send funds
	State    *paych.State
	Vouchers []*VoucherInfo // All vouchers submitted for this channel
}

// Voucher Info is a record of a voucher submitted for a payment channel
type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}
