package api

import (
	"context"

	"gx/ipfs/QmQmhotPUzVrMEWNK3x1R5jQ5ZHWyL7tVUrmRPjrBrvyCb/go-ipfs-files"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Addrs() Addrs
	Import(ctx context.Context, f files.File) ([]address.Address, error)
	Export(ctx context.Context, addrs []address.Address) ([]*types.KeyInfo, error)
}

// Addrs is the interface that defines method to interact with addresses.
type Addrs interface {
	New(ctx context.Context) (address.Address, error)
	Ls(ctx context.Context) ([]address.Address, error)
	Lookup(ctx context.Context, addr address.Address) (peer.ID, error)
}
