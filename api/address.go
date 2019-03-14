package api

import (
	"context"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Import(context.Context, []*types.KeyInfo) ([]address.Address, error)
	Export(context.Context, []address.Address) ([]*types.KeyInfo, error)
}
