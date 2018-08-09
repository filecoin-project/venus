package api

import (
	"context"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

type Miner interface {
	Create(ctx context.Context, fromAddr types.Address, pledge *types.BytesAmount, pid peer.ID, collateral *types.AttoFIL) (types.Address, error)
	UpdatePeerID(ctx context.Context, fromAddr, minerAddr types.Address, newPid peer.ID) (*cid.Cid, error)
	AddAsk(ctx context.Context, fromAddr, minerAddr types.Address, size *types.BytesAmount, price *types.AttoFIL) (*cid.Cid, error)
}
