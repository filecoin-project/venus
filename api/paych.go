package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"github.com/filecoin-project/go-filecoin/address"
)

// Paych is the interface that defines methods to execute payment channel operations.
type Paych interface {
	Ls(ctx context.Context, fromAddr address.Address, payerAddr address.Address) (map[string]*paymentbroker.PaymentChannel, error)
}
