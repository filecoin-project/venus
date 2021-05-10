package paych

import (
	"context"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/paychmgr"

	"golang.org/x/xerrors"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
)

type IPaychan interface {
	// PaychGet creates a payment channel to a provider with a amount of FIL
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @amt: the deposits funds in the payment channel
	PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*ChannelInfo, error)

	// PaychAvailableFunds get the status of an outbound payment channel
	// @pch: payment channel address
	PaychAvailableFunds(ctx context.Context, pch address.Address) (*paychmgr.ChannelAvailableFunds, error)

	// PaychAvailableFundsByFromTo  get the status of an outbound payment channel
	// @from: the payment channel sender
	// @to: he payment channel recipient
	PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*paychmgr.ChannelAvailableFunds, error)

	// PaychGetWaitReady waits until the create channel / add funds message with the sentinel
	// @sentinel: given message CID arrives.
	// @ch: the returned channel address can safely be used against the Manager methods.
	PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (ch address.Address, err error)

	// PaychAllocateLane Allocate late creates a lane within a payment channel so that calls to
	// CreatePaymentVoucher will automatically make vouchers only for the difference in total
	PaychAllocateLane(ctx context.Context, pch address.Address) (uint64, error)

	// PaychNewPayment aggregate vouchers into a new lane
	// @from: the payment channel sender
	// @to: the payment channel recipient
	// @vouchers: the outstanding (non-redeemed) vouchers
	PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []VoucherSpec) (*PaymentInfo, error)

	// PaychList list the addresses of all channels that have been created
	PaychList(ctx context.Context) ([]address.Address, error)

	// PaychStatus get the payment channel status
	// @pch: payment channel address
	PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error)

	// PaychSettle update payment channel status to settle
	// After a settlement period (currently 12 hours) either party to the payment channel can call collect on chain
	// @pch: payment channel address
	PaychSettle(ctx context.Context, pch address.Address) (cid.Cid, error)

	// PaychCollect update payment channel status to collect
	// Collect sends the value of submitted vouchers to the channel recipient (the provider),
	// and refunds the remaining channel balance to the channel creator (the client).
	// @pch: payment channel address
	PaychCollect(ctx context.Context, pch address.Address) (cid.Cid, error)

	// PaychVoucherCheckValid checks if the given voucher is valid (is or could become spendable at some point).
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	// @pch: payment channel address
	// @sv: voucher
	PaychVoucherCheckValid(ctx context.Context, pch address.Address, sv *paych.SignedVoucher) error

	// PaychVoucherCheckSpendable checks if the given voucher is currently spendable
	// @pch: payment channel address
	// @sv: voucher
	PaychVoucherCheckSpendable(ctx context.Context, pch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error)

	// PaychVoucherAdd adds a voucher for an inbound channel.
	// If the channel is not in the store, fetches the channel from state (and checks that
	// the channel To address is owned by the wallet).
	PaychVoucherAdd(ctx context.Context, pch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error)

	// PaychVoucherCreate creates a new signed voucher on the given payment channel
	// with the given lane and amount.  The value passed in is exactly the value
	// that will be used to create the voucher, so if previous vouchers exist, the
	// actual additional value of this voucher will only be the difference between
	// the two.
	// If there are insufficient funds in the channel to create the voucher,
	// returns a nil voucher and the shortfall.
	PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*paychmgr.VoucherCreateResult, error)

	// PaychVoucherList list vouchers in payment channel
	// @pch: payment channel address
	PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error)

	// PaychVoucherSubmit Submit voucher to chain to update payment channel state
	// @pch: payment channel address
	// @sv: voucher in payment channel
	PaychVoucherSubmit(ctx context.Context, pch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error)
}

type paychAPI struct {
	paychMgr *paychmgr.Manager
}

func newPaychAPI(p *paychmgr.Manager) IPaychan {
	return &paychAPI{p}
}

type PaymentInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
	Vouchers     []*paych.SignedVoucher
}

type VoucherSpec struct {
	Amount      big.Int
	TimeLockMin abi.ChainEpoch
	TimeLockMax abi.ChainEpoch
	MinSettle   abi.ChainEpoch

	Extra *paych.ModVerifyParams
}
type ChannelInfo struct {
	Channel      address.Address
	WaitSentinel cid.Cid
}

type ChannelAvailableFunds = paychmgr.ChannelAvailableFunds

type VoucherCreateResult = paychmgr.VoucherCreateResult

type PaychStatus = types.PaychStatus //nolint

func (a *paychAPI) PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*ChannelInfo, error) {
	ch, mcid, err := a.paychMgr.GetPaych(ctx, from, to, amt)
	if err != nil {
		return nil, err
	}

	return &ChannelInfo{
		Channel:      ch,
		WaitSentinel: mcid,
	}, nil
}

func (a *paychAPI) PaychAvailableFunds(ctx context.Context, ch address.Address) (*paychmgr.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFunds(ch)
}

func (a *paychAPI) PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*paychmgr.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFundsByFromTo(from, to)
}

func (a *paychAPI) PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error) {
	return a.paychMgr.GetPaychWaitReady(ctx, sentinel)
}

func (a *paychAPI) PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	return a.paychMgr.AllocateLane(ch)
}

func (a *paychAPI) PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []VoucherSpec) (*PaymentInfo, error) {
	amount := vouchers[len(vouchers)-1].Amount

	// TODO: Fix free fund tracking in PaychGet
	// TODO: validate voucher spec before locking funds
	ch, err := a.PaychGet(ctx, from, to, amount)
	if err != nil {
		return nil, err
	}

	lane, err := a.paychMgr.AllocateLane(ch.Channel)
	if err != nil {
		return nil, err
	}

	svs := make([]*paych.SignedVoucher, len(vouchers))

	for i, v := range vouchers {
		sv, err := a.paychMgr.CreateVoucher(ctx, ch.Channel, paych.SignedVoucher{
			Amount: v.Amount,
			Lane:   lane,

			Extra:           v.Extra,
			TimeLockMin:     v.TimeLockMin,
			TimeLockMax:     v.TimeLockMax,
			MinSettleHeight: v.MinSettle,
		})
		if err != nil {
			return nil, err
		}
		if sv.Voucher == nil {
			return nil, xerrors.Errorf("Could not create voucher - shortfall of %d", sv.Shortfall)
		}

		svs[i] = sv.Voucher
	}

	return &PaymentInfo{
		Channel:      ch.Channel,
		WaitSentinel: ch.WaitSentinel,
		Vouchers:     svs,
	}, nil
}

func (a *paychAPI) PaychList(ctx context.Context) ([]address.Address, error) {
	return a.paychMgr.ListChannels()
}

func (a *paychAPI) PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error) {
	ci, err := a.paychMgr.GetChannelInfo(pch)
	if err != nil {
		return nil, err
	}
	return &types.PaychStatus{
		ControlAddr: ci.Control,
		Direction:   types.PCHDir(ci.Direction),
	}, nil
}

func (a *paychAPI) PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error) {
	return a.paychMgr.Settle(ctx, addr)
}

func (a *paychAPI) PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error) {
	return a.paychMgr.Collect(ctx, addr)
}

func (a *paychAPI) PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error {
	return a.paychMgr.CheckVoucherValid(ctx, ch, sv)
}

func (a *paychAPI) PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	return a.paychMgr.CheckVoucherSpendable(ctx, ch, sv, secret, proof)
}

func (a *paychAPI) PaychVoucherAdd(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	return a.paychMgr.AddVoucherInbound(ctx, ch, sv, proof, minDelta)
}

// PaychVoucherCreate creates a new signed voucher on the given payment channel
// with the given lane and amount.  The value passed in is exactly the value
// that will be used to create the voucher, so if previous vouchers exist, the
// actual additional value of this voucher will only be the difference between
// the two.
// If there are insufficient funds in the channel to create the voucher,
// returns a nil voucher and the shortfall.
func (a *paychAPI) PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*paychmgr.VoucherCreateResult, error) {
	return a.paychMgr.CreateVoucher(ctx, pch, paych.SignedVoucher{Amount: amt, Lane: lane})
}

func (a *paychAPI) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) {
	vi, err := a.paychMgr.ListVouchers(ctx, pch)
	if err != nil {
		return nil, err
	}

	out := make([]*paych.SignedVoucher, len(vi))
	for k, v := range vi {
		out[k] = v.Voucher
	}

	return out, nil
}

func (a *paychAPI) PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	return a.paychMgr.SubmitVoucher(ctx, ch, sv, secret, proof)
}
