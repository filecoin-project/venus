package submodule

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	storagemarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_market_connector"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/storage"
)

// StorageProtocolSubmodule enhances the `Node` with "Storage" protocol capabilities.
type StorageProtocolSubmodule struct {
	StorageAPI *storage.API

	// Storage Market Interfaces
	StorageMiner *storage.Provider
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(ds datastore.Batching, bs blockstore.Blockstore, fs filestore.FileStore, ps piecestore.PieceStore, dt datatransfer.Manager) (StorageProtocolSubmodule, error) {
	connector := storagemarketconnector.NewStorageProviderNodeConnector()
	storageMarketProvider, err := storage.NewProvider(ds, bs, fs, ps, dt, connector)
	if err != nil {
		return StorageProtocolSubmodule{}, err
	}

	return StorageProtocolSubmodule{
		// StorageAPI: nil,
		StorageMiner: storageMarketProvider,
	}, nil
}
