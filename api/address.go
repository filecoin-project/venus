package api

import (
	"context"

	"gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	"gx/ipfs/QmZMWMvWMVKCbHetJ4RgndbuEF1io2UpUxwQwtNjtYPzSC/go-ipfs-files"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Addrs() Addrs
	Balance(ctx context.Context, addr address.Address) (*types.AttoFIL, error)
	Import(ctx context.Context, f files.File) ([]address.Address, error)
	Export(ctx context.Context, addrs []address.Address) ([]*types.KeyInfo, error)
}

// Addrs is the interface that defines method to interact with addresses.
type Addrs interface {
	New(ctx context.Context) (address.Address, error)
	Ls(ctx context.Context) ([]address.Address, error)
	Lookup(ctx context.Context, addr address.Address) (peer.ID, error)
}
