package settler

import (
	"context"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/app/submodule/chain/cst"
	api "github.com/filecoin-project/venus/app/submodule/paych"
	"github.com/filecoin-project/venus/pkg/paychmgr"
	"github.com/filecoin-project/venus/pkg/specactors/builtin/paych"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
)

type SettlerAPI interface {
	PaychList(context.Context) ([]address.Address, error)
	PaychStatus(ctx context.Context, pch address.Address) (*api.PaychStatus, error)
	PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error)
	PaychVoucherList(context.Context, address.Address) ([]*paych.SignedVoucher, error)
	PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error)
	StateWaitMsg(ctx context.Context, cid cid.Cid, confidence abi.ChainEpoch) (*cst.MsgLookup, error)
}

type settlerAPI struct {
	mgr   *paychmgr.Manager
	ciAPI *chain.ChainInfoAPI
}

func NewSetterAPI(mgr *paychmgr.Manager, chainInfoAPI *chain.ChainInfoAPI) SettlerAPI {
	return &settlerAPI{mgr, chainInfoAPI}
}

func (o *settlerAPI) PaychList(context.Context) ([]address.Address, error) {
	return o.mgr.ListChannels()
}

func (o *settlerAPI) PaychStatus(ctx context.Context, pch address.Address) (*api.PaychStatus, error) {
	ci, err := o.mgr.GetChannelInfo(pch)
	if err != nil {
		return nil, err
	}
	return &api.PaychStatus{
		ControlAddr: ci.Control,
		Direction:   types.PCHDir(ci.Direction),
	}, nil
}
func (o *settlerAPI) PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error) {
	return o.mgr.CheckVoucherSpendable(ctx, ch, sv, secret, proof)
}
func (o *settlerAPI) PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error) {
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
func (o *settlerAPI) PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error) {
	return o.mgr.SubmitVoucher(ctx, ch, sv, secret, proof)
}
func (o *settlerAPI) StateWaitMsg(ctx context.Context, cid cid.Cid, confidence abi.ChainEpoch) (*cst.MsgLookup, error) {
	return o.ciAPI.StateWaitMsg(ctx, cid, confidence)
}
