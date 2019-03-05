package api

import (
	"context"

	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Miner is the interface that defines methods to manage miner operations.
type Miner interface {
	Create(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (address.Address, error)
}
