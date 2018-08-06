package api

import (
	"context"

	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

type AddressAPI interface {
	Addrs() AddrsAPI
	Balance(ctx context.Context, addr types.Address) (*types.AttoFIL, error)
}

type AddrsAPI interface {
	New(ctx context.Context) (types.Address, error)
	Ls(ctx context.Context) ([]types.Address, error)
	Lookup(ctx context.Context, addr types.Address) (peer.ID, error)
}
