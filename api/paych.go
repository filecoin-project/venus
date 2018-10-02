package api

import (
	"context"

	cid "gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Paych is the interface that defines methods to execute payment channel operations.
type Paych interface {
	Create(ctx context.Context, fromAddr, target address.Address, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error)
	Ls(ctx context.Context, fromAddr, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error)
	Voucher(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, amount *types.AttoFIL) (string, error)
	Redeem(ctx context.Context, fromAddr address.Address, voucherRaw string) (*cid.Cid, error)
	Reclaim(ctx context.Context, fromAddr address.Address, channel *types.ChannelID) (*cid.Cid, error)
	Close(ctx context.Context, fromAddr address.Address, voucherRaw string) (*cid.Cid, error)
	Extend(ctx context.Context, fromAddr address.Address, channel *types.ChannelID, eol *types.BlockHeight, amount *types.AttoFIL) (*cid.Cid, error)
}
