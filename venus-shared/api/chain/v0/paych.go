package v0

import (
	"context"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/ipfs/go-cid"
)

type IPaychan interface {
	// PaychGet creates a payment channel to a provider with a amount of FIL
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @amt: the deposits funds in the payment channel
	PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*types.ChannelInfo, error) //perm:sign
	// PaychAvailableFunds get the status of an outbound payment channel
	// @pch: payment channel address
	PaychAvailableFunds(ctx context.Context, ch address.Address) (*types.ChannelAvailableFunds, error) //perm:sign
	// PaychAvailableFundsByFromTo  get the status of an outbound payment channel
	// @from: the payment channel sender
	// @to: he payment channel recipient
	PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*types.ChannelAvailableFunds, error) //perm:sign
	// PaychGetWaitReady waits until the create channel / add funds message with the sentinel
	// @sentinel: given message CID arrives.
	// @ch: the returned channel address can safely be used against the Manager methods.
	PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error) //perm:sign
	// PaychAllocateLane Allocate late creates a lane within a payment channel so that calls to
	// CreatePaymentVoucher will automatically make vouchers only for the difference in total
	PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error) //perm:sign
	// PaychNewPayment aggregate vouchers into a new lane
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @vouchers: the outstanding (non-redeemed) vouchers
	PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []types.VoucherSpec) (*types.PaymentInfo, error) //perm:sign
	// PaychList list the addresses of all channels that have been created
	PaychList(ctx context.Context) ([]address.Address, error) //perm:read
	// PaychStatus get the payment channel status
	// @pch: payment channel address
	PaychStatus(ctx context.Context, pch address.Address) (*types.Status, error) //perm:read
	// PaychSettle update payment channel status to settle
	// After a settlement period (currently 12 hours) either party to the payment channel can call collect on chain
	// @pch: payment channel address
	PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error) //perm:sign
	// PaychCollect update payment channel status to collect
	// Collect sends the value of submitted vouchers to the channel recipient (the provider),
	// and refunds the remaining channel balance to the channel creator (the client).
	// @pch: payment channel address
	PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error) //perm:sign

	// PaychVoucherCheckValid checks if the given voucher is valid (is or could become spendable at some point).
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	// @pch: payment channel address
	// @sv: voucher
	PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *types.SignedVoucher) error //perm:read
	// PaychVoucherCheckSpendable checks if the given voucher is currently spendable
	// @pch: payment channel address
	// @sv: voucher
	PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *types.SignedVoucher, secret []byte, proof []byte) (bool, error) //perm:read
	// PaychVoucherAdd adds a voucher for an inbound channel.
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	PaychVoucherAdd(ctx context.Context, ch address.Address, sv *types.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) //perm:write
	// PaychVoucherCreate creates a new signed voucher on the given payment channel
	// with the given lane and amount.  The value passed in is exactly the value
	// that will be used to create the voucher, so if previous vouchers exist, the
	// actual additional value of this voucher will only be the difference between
	// the two.
	// If there are insufficient funds in the channel to create the voucher,
	// returns a nil voucher and the shortfall.
	PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*types.VoucherCreateResult, error) //perm:sign
	// PaychVoucherList list vouchers in payment channel
	// @pch: payment channel address
	PaychVoucherList(ctx context.Context, pch address.Address) ([]*types.SignedVoucher, error) //perm:write
	// PaychVoucherSubmit Submit voucher to chain to update payment channel state
	// @pch: payment channel address
	// @sv: voucher in payment channel
	PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *types.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) //perm:sign
}
