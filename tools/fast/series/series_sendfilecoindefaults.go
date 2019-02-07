package series

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

func SendFilecoinDefaults(ctx context.Context, from, to *fast.Filecoin, value int) error {
	var fromAddr address.Address
	if err := from.ConfigGet(ctx, "wallet.defaultAddress", &fromAddr); err != nil {
		return err
	}

	var toAddr address.Address
	if err := to.ConfigGet(ctx, "wallet.defaultAddress", &toAddr); err != nil {
		return err
	}

	mcid, err := from.MessageSend(ctx, toAddr, "", fast.AOValue(value), fast.AOFromAddr(fromAddr), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return err
	}

	if _, err := to.MessageWait(ctx, mcid); err != nil {
		return err
	}

	return nil
}
