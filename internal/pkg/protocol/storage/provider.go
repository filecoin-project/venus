package storage

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	m "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// Provider is
type Provider struct{}

// NewProvider consummates deals, interacting (indirectly) with the storage
// market actor, sector subsystems, and blockchain.
func NewProvider(ds datastore.Batching, bs blockstore.Blockstore, fs filestore.FileStore, ps piecestore.PieceStore, dt datatransfer.Manager, sp storagemarket.StorageProviderNode) (*Provider, error) {
	_, err := m.NewProvider(ds, bs, fs, ps, dt, sp)
	if err != nil {
		return nil, err
	}

	return &Provider{}, nil // nolint:govet
}
