package market

import (
	"context"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/app/submodule/mpool"
	"github.com/filecoin-project/venus/pkg/market"
	"github.com/filecoin-project/venus/pkg/specactors"
	bimarket "github.com/filecoin-project/venus/pkg/specactors/builtin/market"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-address"
)

type MarketAPI interface {
	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt big.Int) (cid.Cid, error)
	MarketGetReserved(ctx context.Context, addr address.Address) (big.Int, error)
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt big.Int) (cid.Cid, error)
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt big.Int) error
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt big.Int) (cid.Cid, error)
}
type marketAPI struct {
	*mpool.MessagePoolAPI
	fmgr *market.FundManager
}

func newMarketAPI(mp *mpool.MessagePoolAPI, fmgr *market.FundManager) MarketAPI {
	return &marketAPI{mp, fmgr}
}
func (a *marketAPI) MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt big.Int) (cid.Cid, error) {
	params, err := specactors.SerializeParams(&addr)
	if err != nil {
		return cid.Undef, err
	}

	smsg, aerr := a.MpoolPushMessage(ctx, &types.UnsignedMessage{
		To:     bimarket.Address,
		From:   wallet,
		Value:  amt,
		Method: bimarket.Methods.AddBalance,
		Params: params,
	}, nil)

	if aerr != nil {
		return cid.Undef, aerr
	}
	smsgCid, aerr := smsg.Cid()
	if aerr != nil {
		return cid.Undef, aerr
	}
	return smsgCid, nil
}

func (a *marketAPI) MarketGetReserved(ctx context.Context, addr address.Address) (big.Int, error) {
	return a.fmgr.GetReserved(addr), nil
}

func (a *marketAPI) MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt big.Int) (cid.Cid, error) {
	return a.fmgr.Reserve(ctx, wallet, addr, amt)
}

func (a *marketAPI) MarketReleaseFunds(ctx context.Context, addr address.Address, amt big.Int) error {
	return a.fmgr.Release(addr, amt)
}

func (a *marketAPI) MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt big.Int) (cid.Cid, error) {
	return a.fmgr.Withdraw(ctx, wallet, addr, amt)
}
