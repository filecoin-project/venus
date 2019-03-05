package api

import (
	"context"
	"math/big"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Miner is the interface that defines methods to manage miner operations.
type Miner interface {
	Create(ctx context.Context, fromAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, pledge uint64, pid peer.ID, collateral *types.AttoFIL) (address.Address, error)
	UpdatePeerID(ctx context.Context, fromAddr, minerAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, newPid peer.ID) (cid.Cid, error)
	AddAsk(ctx context.Context, fromAddr, minerAddr address.Address, gasPrice types.AttoFIL, gasLimit types.GasUnits, price *types.AttoFIL, expiry *big.Int) (cid.Cid, error)
	GetOwner(ctx context.Context, minerAddr address.Address) (address.Address, error)
}
