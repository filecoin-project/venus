package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Paych is the interface that defines methods to execute payment channel operations.
type Paych interface {
	Ls(ctx context.Context, fromAddr address.Address, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error)
	Voucher(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, amount *types.AttoFIL, validAt *types.BlockHeight) (string, error)
}
