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

// Miner is
type Miner struct{}

// NewMiner is
func NewMiner(ds datastore.Batching, bs blockstore.Blockstore, fs filestore.FileStore, ps piecestore.PieceStore, dt datatransfer.Manager, sp storagemarket.StorageProviderNode) (*Miner, error) {
	_, err := m.NewProvider(ds, bs, fs, ps, dt, sp)
	if err != nil {
		return nil, err
	}

	return &Miner{}, nil // nolint:govet
}
