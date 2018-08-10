package api

import (
	"context"

	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit/files"
	"gx/ipfs/QmdVrMn1LhB4ybb8hMVaMLXnA8XRSewMnK6YqXKXoTcRvN/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Addrs() Addrs
	Balance(ctx context.Context, addr types.Address) (*types.AttoFIL, error)
	Import(ctx context.Context, f files.File) error
	Export(ctx context.Context, addrs []types.Address) ([]*types.KeyInfo, error)
}

// Addrs is the interface that defines method to interact with addresses.
type Addrs interface {
	New(ctx context.Context) (types.Address, error)
	Ls(ctx context.Context) ([]types.Address, error)
	Lookup(ctx context.Context, addr types.Address) (peer.ID, error)
}
