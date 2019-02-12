package api

import (
	"context"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Paych is the interface that defines methods to execute payment channel operations.
type Paych interface {
	Create(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, target address.Address, eol *types.BlockHeight, amount *types.AttoFIL) (cid.Cid, error)
	Ls(ctx context.Context, fromAddr address.Address, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error)
	Voucher(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, amount *types.AttoFIL, validAt *types.BlockHeight) (string, error)
	Redeem(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, voucherRaw string) (cid.Cid, error)
	Reclaim(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, channel *types.ChannelID) (cid.Cid, error)
	Close(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, voucherRaw string) (cid.Cid, error)
	Extend(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (cid.Cid, error)
}
