package paych

import (
	"context"

	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/specs-actors/actors/builtin/paych"

	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/types"
)

type paychAPI struct {
	paychMgr *paychmgr.Manager
}

func newPaychAPI(p *paychmgr.Manager) apiface.IPaychan {
	return &paychAPI{p}
}

type ChannelAvailableFunds = apitypes.ChannelAvailableFunds

type PaychStatus = types.PaychStatus //nolint

func (a *paychAPI) PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*apitypes.ChannelInfo, error) {
	ch, mcid, err := a.paychMgr.GetPaych(ctx, from, to, amt)
	if err != nil {
		return nil, err
	}

	return &apitypes.ChannelInfo{
		Channel:      ch,
		WaitSentinel: mcid,
	}, nil
}

func (a *paychAPI) PaychAvailableFunds(ctx context.Context, ch address.Address) (*apitypes.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFunds(ch)
}

func (a *paychAPI) PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*apitypes.ChannelAvailableFunds, error) {
	return a.paychMgr.AvailableFundsByFromTo(from, to)
}

func (a *paychAPI) PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error) {
	return a.paychMgr.GetPaychWaitReady(ctx, sentinel)
}

func (a *paychAPI) PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error) {
	return a.paychMgr.AllocateLane(ch)
}

func (a *paychAPI) PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []apitypes.VoucherSpec) (*apitypes.PaymentInfo, error) {
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

	return &apitypes.PaymentInfo{
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
func (a *paychAPI) PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*apitypes.VoucherCreateResult, error) {
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
