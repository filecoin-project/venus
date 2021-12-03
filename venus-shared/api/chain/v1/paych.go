package v1

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/paych"
)

type IPaychan interface {
	// PaychGet creates a payment channel to a provider with a amount of FIL
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @amt: the deposits funds in the payment channel
	// Rule[perm:read]
	PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*paych.ChannelInfo, error)
	// PaychAvailableFunds get the status of an outbound payment channel
	// @pch: payment channel address
	// Rule[perm:read]
	PaychAvailableFunds(ctx context.Context, ch address.Address) (*ChannelAvailableFunds, error)
	// PaychAvailableFundsByFromTo  get the status of an outbound payment channel
	// @from: the payment channel sender
	// @to: he payment channel recipient
	// Rule[perm:read]
	PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*ChannelAvailableFunds, error)
	// PaychGetWaitReady waits until the create channel / add funds message with the sentinel
	// @sentinel: given message CID arrives.
	// @ch: the returned channel address can safely be used against the Manager methods.
	// Rule[perm:read]
	PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error)
	// PaychAllocateLane Allocate late creates a lane within a payment channel so that calls to
	// CreatePaymentVoucher will automatically make vouchers only for the difference in total
	// Rule[perm:read]
	PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error)
	// PaychNewPayment aggregate vouchers into a new lane
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @vouchers: the outstanding (non-redeemed) vouchers
	// Rule[perm:read]
	PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []paych.VoucherSpec) (*paych.PaymentInfo, error)
	// PaychList list the addresses of all channels that have been created
	// Rule[perm:read]
	PaychList(ctx context.Context) ([]address.Address, error)
	// PaychStatus get the payment channel status
	// @pch: payment channel address
	// Rule[perm:read]
	PaychStatus(ctx context.Context, pch address.Address) (*paych.Status, error)
	// PaychSettle update payment channel status to settle
	// After a settlement period (currently 12 hours) either party to the payment channel can call collect on chain
	// @pch: payment channel address
	// Rule[perm:read]
	PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error)
	// PaychCollect update payment channel status to collect
	// Collect sends the value of submitted vouchers to the channel recipient (the provider),
	// and refunds the remaining channel balance to the channel creator (the client).
	// @pch: payment channel address
	// Rule[perm:read]
	PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error)

	// PaychVoucherCheckValid checks if the given voucher is valid (is or could become spendable at some point).
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	// @pch: payment channel address
	// @sv: voucher
	// Rule[perm:read]
	PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error
	// PaychVoucherCheckSpendable checks if the given voucher is currently spendable
	// @pch: payment channel address
	// @sv: voucher
	// Rule[perm:read]
	PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error)
	// PaychVoucherAdd adds a voucher for an inbound channel.
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	// Rule[perm:read]
	PaychVoucherAdd(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error)
	// PaychVoucherCreate creates a new signed voucher on the given payment channel
	// with the given lane and amount.  The value passed in is exactly the value
	// that will be used to create the voucher, so if previous vouchers exist, the
	// actual additional value of this voucher will only be the difference between
	// the two.
	// If there are insufficient funds in the channel to create the voucher,
	// returns a nil voucher and the shortfall.
	// Rule[perm:read]
	PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*paych.VoucherCreateResult, error)
	// PaychVoucherList list vouchers in payment channel
	// @pch: payment channel address
	// Rule[perm:read]
	PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error)
	// PaychVoucherSubmit Submit voucher to chain to update payment channel state
	// @pch: payment channel address
	// @sv: voucher in payment channel
	// Rule[perm:read]
	PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error)
}
