package api

import (
	"context"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit/files"

	"github.com/filecoin-project/go-filecoin/types"
)

// Address is the interface that defines methods to manage Filecoin addresses and wallets.
type Address interface {
	Addrs() Addrs
	Balance(ctx context.Context, addr types.Address) (*types.AttoFIL, error)
	Import(ctx context.Context, f files.File) ([]types.Address, error)
	Export(ctx context.Context, addrs []types.Address) ([]*types.KeyInfo, error)
}

// Addrs is the interface that defines method to interact with addresses.
type Addrs interface {
	New(ctx context.Context) (types.Address, error)
	Ls(ctx context.Context) ([]types.Address, error)
	Lookup(ctx context.Context, addr types.Address) (peer.ID, error)
}
