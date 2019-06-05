package series

import (
	"context"
	"math/big"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/tools/fast"
)

// SendFilecoinFromDefault will send the `value` of FIL from the default wallet
// address, per the config of the `node`, to the provided address `addr` and
// wait for the message to showup on chain.
// The waiting node is the sender, this does not guarantee that the message has
// been received by the targeted node of addr.
func SendFilecoinFromDefault(ctx context.Context, node *fast.Filecoin, addr address.Address, value int) (cid.Cid, error) {
	var walletAddr address.Address
	if err := node.ConfigGet(ctx, "wallet.defaultAddress", &walletAddr); err != nil {
		return cid.Undef, err
	}

	mcid, err := node.MessageSend(ctx, addr, "", fast.AOValue(value), fast.AOFromAddr(walletAddr), fast.AOPrice(big.NewFloat(1.0)), fast.AOLimit(300))
	if err != nil {
		return cid.Undef, err
	}

	CtxMiningOnce(ctx)

	if _, err := node.MessageWait(ctx, mcid); err != nil {
		return cid.Undef, err
	}

	return mcid, nil
}
