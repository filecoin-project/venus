package v0api

import (
	"context"

	"github.com/filecoin-project/go-address"

	v0api "github.com/filecoin-project/venus/venus-shared/api/chain/v0"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
)

type WrapperV1IPaych struct {
	v1api.IPaychan
}

func (w *WrapperV1IPaych) PaychGet(ctx context.Context, from, to address.Address, amt types.BigInt) (*types.ChannelInfo, error) {
	return w.PaychFund(ctx, from, to, amt)
}

var _ v0api.IPaychan = &WrapperV1IPaych{}
