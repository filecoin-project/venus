package submodule

import (
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	iface "github.com/filecoin-project/go-fil-markets/storagemarket"
	impl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/filecoin-project/go-statestore"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"

	storagemarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_market_connector"
)

// StorageProtocolSubmodule enhances the node with storage protocol
// capabilities.
type StorageProtocolSubmodule struct {
	StorageClient   iface.StorageClient
	StorageProvider iface.StorageProvider
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(h host.Host, ds datastore.Batching, bs blockstore.Blockstore, fs filestore.FileStore, ps piecestore.PieceStore, dt datatransfer.Manager, dsc *discovery.Local, dls *statestore.StateStore) (StorageProtocolSubmodule, error) {
	panic("TODO: go-fil-markets integration")

	pnode := storagemarketconnector.NewStorageProviderNodeConnector()
	cnode := storagemarketconnector.NewStorageClientNodeConnector()

	provider, err := impl.NewProvider(ds, bs, fs, ps, dt, pnode)
	if err != nil {
		return StorageProtocolSubmodule{}, err
	}

	return StorageProtocolSubmodule{
		StorageClient:   impl.NewClient(h, bs, fs, dt, dsc, dls, cnode),
		StorageProvider: provider,
	}, nil
}
