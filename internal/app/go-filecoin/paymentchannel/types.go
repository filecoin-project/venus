package paymentchannel

import (
	"github.com/ipfs/go-cid"
	"math/big"
	"reflect"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	big2 "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
)

//go:generate cbor-gen-for ChannelInfo VoucherInfo

// The key for the store is the payment channel's "robust" or "unique" address

// ChannelInfo is the primary payment channel record
// UniqueAddr: aka RobustAddr, used externally to refer to payment channel
// Duplicated in data due to need to check for existing payment channel since you can't
// iterate over key/value pairs in statestore at present
type ChannelInfo struct {
	UniqueAddr    address.Address
	From          address.Address
	To            address.Address
	NextLane      uint64
	NextNonce     uint64
	// Amount added to the channel.
	// Note: This amount is only used by GetPaych to keep track of how much
	// has locally been added to the channel. It should reflect the channel's
	// Balance on chain as long as all operations occur on the same datastore.
	Amount        big2.Int
	PendingAmount big2.Int // PendingAmount is the amount that we're awaiting confirmation of
	CreateMsg     *cid.Cid // CreateMsg is the CID of a pending create message (while waiting for confirmation)
	AddFundsMsg   *cid.Cid // AddFundsMsg is the CID of a pending add funds message (while waiting for confirmation)
	Vouchers      []*VoucherInfo // All vouchers submitted for this channel
}

// IsZero returns whether it is a zeroed/blank ChannelInfo
func (ci *ChannelInfo) IsZero() bool {
	return ci.UniqueAddr.Empty() && ci.To.Empty() && ci.From.Empty() &&
		ci.NextLane == 0 && len(ci.Vouchers) == 0
}

// HasVoucher returns true if `voucher` is already in `info`
func (ci *ChannelInfo) HasVoucher(voucher *paych.SignedVoucher) bool {
	for _, v := range ci.Vouchers {
		if reflect.DeepEqual(*v.Voucher, *voucher) {
			return true
		}
	}
	return false
}

// LargestVoucherAmount returns the largest stored voucher amount
func (ci *ChannelInfo) LargestVoucherAmount() abi.TokenAmount {
	res := abi.NewTokenAmount(0)
	for _, v := range ci.Vouchers {
		if v.Voucher.Amount.GreaterThan(res) {
			res = v.Voucher.Amount
		}
	}
	return res
}

// VoucherInfo is a record of a voucher submitted for a payment channel
type VoucherInfo struct {
	Voucher *paych.SignedVoucher
	Proof   []byte
}

type ChannelAvailableFunds struct {
	// Channel is the address of the channel
	Channel *address.Address
	// From is the from address of the channel (channel creator)
	From address.Address
	// To is the to address of the channel
	To address.Address
	// ConfirmedAmt is the amount of funds that have been confirmed on-chain
	// for the channel
	ConfirmedAmt big.Int
	// PendingAmt is the amount of funds that are pending confirmation on-chain
	PendingAmt big.Int
	// PendingWaitSentinel can be used with PaychGetWaitReady to wait for
	// confirmation of pending funds
	PendingWaitSentinel *cid.Cid
	// QueuedAmt is the amount that is queued up behind a pending request
	QueuedAmt big.Int
	// VoucherRedeemedAmt is the amount that is redeemed by vouchers on-chain
	// and in the local datastore
	VoucherReedeemedAmt big.Int
}
