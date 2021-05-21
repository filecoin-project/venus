package apiface

import (
	"context"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/app/submodule/apitypes"
	"github.com/filecoin-project/venus/pkg/types"

	"github.com/filecoin-project/specs-actors/actors/builtin/paych"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
)

type IPaychan interface {
	// Rule[perm:read]
	PaychGet(ctx context.Context, from, to address.Address, amt big.Int) (*apitypes.ChannelInfo, error)
	// Rule[perm:read]
	PaychAvailableFunds(ctx context.Context, ch address.Address) (*apitypes.ChannelAvailableFunds, error)
	// Rule[perm:read]
	PaychAvailableFundsByFromTo(ctx context.Context, from, to address.Address) (*apitypes.ChannelAvailableFunds, error)
	// Rule[perm:read]
	PaychGetWaitReady(ctx context.Context, sentinel cid.Cid) (address.Address, error)
	// Rule[perm:read]
	PaychAllocateLane(ctx context.Context, ch address.Address) (uint64, error)
	// Rule[perm:read]
	PaychNewPayment(ctx context.Context, from, to address.Address, vouchers []apitypes.VoucherSpec) (*apitypes.PaymentInfo, error)
	// Rule[perm:read]
	PaychList(ctx context.Context) ([]address.Address, error)
	// Rule[perm:read]
	PaychStatus(ctx context.Context, pch address.Address) (*types.PaychStatus, error)
	// Rule[perm:read]
	PaychSettle(ctx context.Context, addr address.Address) (cid.Cid, error)
	// Rule[perm:read]
	PaychCollect(ctx context.Context, addr address.Address) (cid.Cid, error)
	// Rule[perm:read]
	PaychVoucherCheckValid(ctx context.Context, ch address.Address, sv *paych.SignedVoucher) error
	// Rule[perm:read]
	PaychVoucherCheckSpendable(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (bool, error)
	// Rule[perm:read]
	PaychVoucherAdd(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, proof []byte, minDelta big.Int) (big.Int, error)
	// Rule[perm:read]
	PaychVoucherCreate(ctx context.Context, pch address.Address, amt big.Int, lane uint64) (*apitypes.VoucherCreateResult, error)
	// Rule[perm:read]
	PaychVoucherList(ctx context.Context, pch address.Address) ([]*paych.SignedVoucher, error)
	// Rule[perm:read]
	PaychVoucherSubmit(ctx context.Context, ch address.Address, sv *paych.SignedVoucher, secret []byte, proof []byte) (cid.Cid, error)
}
