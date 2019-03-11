package api

import (
	"context"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Import(ctx context.Context, f files.File) ([]address.Address, error)
	Export(ctx context.Context, addrs []address.Address) ([]*types.KeyInfo, error)
}
