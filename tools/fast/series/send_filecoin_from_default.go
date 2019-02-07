package series

import (
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// SendFilecoinFromDefault will send the `value` of FIL from the default wallet
// address, per the config of the `node`, to the provided address `addr` and
// wait for the message to showup on chain.
func SendFilecoinFromDefault(ctx context.Context, node *fast.Filecoin, addr address.Address, value int) error {
	var walletAddr address.Address
	if err := node.ConfigGet(ctx, "wallet.defaultAddress", &walletAddr); err != nil {
		return err
	}

	mcid, err := node.MessageSend(ctx, addr, "", fast.AOValue(value), fast.AOFromAddr(walletAddr), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return err
	}

	if _, err := node.MessageWait(ctx, mcid); err != nil {
		return err
	}

	return nil
}
