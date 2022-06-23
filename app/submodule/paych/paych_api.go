package paych

import (
	"context"
	"fmt"

	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v8/paych"

	"github.com/filecoin-project/venus/pkg/paychmgr"
)

type PaychAPI struct { //nolint
	paychMgr *paychmgr.Manager
}

func NewPaychAPI(p *paychmgr.Manager) *PaychAPI {
	return &PaychAPI{p}
}

func (a *PaychAPI) PaychGet(ctx context.Context, from, to address.Address, amt types.BigInt, opts types.PaychGetOpts) (*types.ChannelInfo, error) {
	ch, mcid, err := a.paychMgr.GetPaych(ctx, from, to, amt, paychmgr.GetOpts{
		Reserve:  true,
		OffChain: opts.OffChain,
	})
	if err != nil {
		return nil, err
	}

	return &types.ChannelInfo{
		Channel:      ch,
		WaitSentinel: mcid,
	}, nil
}

func (a *PaychAPI) PaychFund(ctx context.Context, from, to address.Address, amt types.BigInt) (*types.ChannelInfo, error) {
	ch, mcid, err := a.paychMgr.GetPaych(ctx, from, to, amt, paychmgr.GetOpts{
		Reserve:  false,
		OffChain: false,
	})
	if err != nil {
		return nil, err
	}

	return &types.ChannelInfo{
		Channel:      ch,
		WaitSentinel: mcid,
	}, nil
}

func (a *PaychAPI) PaychAvailableFunds(ctx context.Context, ch address.Address) (*types.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFunds(ctx, ch)
}

func (a *PaychAPI) PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*types.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFundsByFromTo(ctx, from, to)
}

func (a *PaychAPI) PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error) {
	return a.paychMgr.GetPaychWaitReady(ctx, sentinel)
}

func (a *PaychAPI) PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	return a.paychMgr.AllocateLane(ctx, ch)
}

func (a *PaychAPI) PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []types.VoucherSpec) (*types.PaymentInfo, error) {
	amount := vouchers[len(vouchers)-1].Amount

	// TODO: Fix free fund tracking in PaychGet
	// TODO: validate voucher spec before locking funds
	ch, err := a.PaychGet(ctx, from, to, amount, types.PaychGetOpts{OffChain: false})
	if err != nil {
		return nil, err
	}

	lane, err := a.paychMgr.AllocateLane(ctx, ch.Channel)
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
			return nil, fmt.Errorf("could not create voucher - shortfall of %d", sv.Shortfall)
		}

		svs[i] = sv.Voucher
	}

	return &types.PaymentInfo{
		Channel:      ch.Channel,
		WaitSentinel: ch.WaitSentinel,
		Vouchers:     svs,
	}, nil
}

func (a *PaychAPI) PaychList(ctx context.Context) ([]address.Address, error) {
	return a.paychMgr.ListChannels(ctx)
}

func (a *PaychAPI) PaychStatus(ctx context.Context, pch address.Address) (*types.Status, error) {
	ci, err := a.paychMgr.GetChannelInfo(ctx, pch)
	if err != nil {
		return nil, err
	}
	return &types.Status{
		ControlAddr: ci.Control,
		Direction:   types.PCHDir(ci.Direction),
	}, nil
}

func (a *PaychAPI) PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error) {
	return a.paychMgr.Settle(ctx, addr)
}

func (a *PaychAPI) PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error) {
	return a.paychMgr.Collect(ctx, addr)
}

func (a *PaychAPI) PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error {
	return a.paychMgr.CheckVoucherValid(ctx, ch, sv)
}

func (a *PaychAPI) PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	return a.paychMgr.CheckVoucherSpendable(ctx, ch, sv, secret, proof)
}

func (a *PaychAPI) PaychVoucherAdd(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error) {
	return a.paychMgr.AddVoucherInbound(ctx, ch, sv, proof, minDelta)
}

// PaychVoucherCreate creates a new signed voucher on the given payment channel
// with the given lane and amount.  The value passed in is exactly the value
// that will be used to create the voucher, so if previous vouchers exist, the
// actual additional value of this voucher will only be the difference between
// the two.
// If there are insufficient funds in the channel to create the voucher,
// returns a nil voucher and the shortfall.
func (a *PaychAPI) PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*types.VoucherCreateResult, error) {
	return a.paychMgr.CreateVoucher(ctx, pch, paych.SignedVoucher{Amount: amt, Lane: lane})
}

func (a *PaychAPI) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) {
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

func (a *PaychAPI) PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	return a.paychMgr.SubmitVoucher(ctx, ch, sv, secret, proof)
}
