package series

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// SendFilecoinDefaults sends the `value` amount of fil from the default wallet
// address of the `from` node to the `to` node's default wallet, and waits for the
// message to be received by the `to` node.
func SendFilecoinDefaults(ctx context.Context, from, to *fast.Filecoin, value int) error {
	var toAddr address.Address
	if err := to.ConfigGet(ctx, "wallet.defaultAddress", &toAddr); err != nil {
		return err
	}

	mcid, err := SendFilecoinFromDefault(ctx, from, toAddr, value)
	if err != nil {
		return err
	}

	if _, err := to.MessageWait(ctx, mcid); err != nil {
		return err
	}

	return nil
}
