package settler

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/apiface"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type Settler interface {
	PaychList(context.Context) ([]address.Address, error)
	PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error)
	PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error)
	PaychVoucherList(context.Context, address.Address) ([]*paych.SignedVoucher, error)
	PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error)
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, limit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error)
}

type settler struct {
	mgr   *paychmgr.Manager
	ciAPI apiface.IChainInfo
}

func NewSetter(mgr *paychmgr.Manager, chainInfoAPI apiface.IChainInfo) Settler {
	return &settler{mgr, chainInfoAPI}
}

func (o *settler) PaychList(context.Context) ([]address.Address, error) {
	return o.mgr.ListChannels()
}

func (o *settler) PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error) {
	ci, err := o.mgr.GetChannelInfo(pch)
	if err != nil {
		return nil, err
	}
	return &types.PaychStatus{
		ControlAddr: ci.Control,
		Direction:   types.PCHDir(ci.Direction),
	}, nil
}
func (o *settler) PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	return o.mgr.CheckVoucherSpendable(ctx, ch, sv, secret, proof)
}
func (o *settler) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) {
	vi, err := o.mgr.ListVouchers(ctx, pch)
	if err != nil {
		return nil, err
	}

	out := make([]*paych.SignedVoucher, len(vi))
	for k, v := range vi {
		out[k] = v.Voucher
	}
	return out, nil
}
func (o *settler) PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	return o.mgr.SubmitVoucher(ctx, ch, sv, secret, proof)
}
func (o *settler) StateWaitMsg(ctx context.Context, cid cid.Cid, confidence uint64, lookbackLimit abi.ChainEpoch, allowReplaced bool) (*apitypes.MsgLookup, error) {
	return o.ciAPI.StateWaitMsg(ctx, cid, confidence, lookbackLimit, allowReplaced)
}
