package submodule

import (
	"context"

	"github.com/filecoin-project/go-address"
	graphsyncimpl "github.com/filecoin-project/go-data-transfer/impl/graphsync"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/storagemarket"
	impl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	storagemarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	fcaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
	"github.com/ipfs/go-datastore"
	graphsync "github.com/ipfs/go-graphsync/impl"
	"github.com/ipfs/go-graphsync/ipldbridge"
	gsnet "github.com/ipfs/go-graphsync/network"
	gsstoreutil "github.com/ipfs/go-graphsync/storeutil"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"
)

// StorageProtocolSubmodule enhances the node with storage protocol
// capabilities.
type StorageProtocolSubmodule struct {
	StorageClient   iface.StorageClient
	StorageProvider iface.StorageProvider
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(
	ctx context.Context,
	minerAddr fcaddr.Address,
	c *ChainSubmodule,
	m *MessagingSubmodule,
	mw *msg.Waiter,
	pm piecemanager.PieceManager,
	wlt *wallet.Wallet,
	h host.Host,
	ds datastore.Batching,
	bs blockstore.Blockstore,
	repoPath string,
	wg storagemarketconnector.WorkerGetter) (*StorageProtocolSubmodule, error) {

	ma, err := address.NewFromBytes(minerAddr.Bytes())
	if err != nil {
		return nil, err
	}

	pnode := storagemarketconnector.NewStorageProviderNodeConnector(ma, c.State, m.Outbox, mw, pm, wg, wlt)
	cnode := storagemarketconnector.NewStorageClientNodeConnector(c.State, mw, wlt)

	pieceStagingPath, err := paths.PieceStagingDir(repoPath)
	if err != nil {
		return nil, err
	}

	fs, err := filestore.NewLocalFileStore(filestore.OsPath(pieceStagingPath))
	if err != nil {
		return nil, err
	}

	graphsyncNetwork := gsnet.NewFromLibp2pHost(h)
	bridge := ipldbridge.NewIPLDBridge()
	loader := gsstoreutil.LoaderForBlockstore(bs)
	storer := gsstoreutil.StorerForBlockstore(bs)
	gsync := graphsync.New(ctx, graphsyncNetwork, bridge, loader, storer)

	dt := graphsyncimpl.NewGraphSyncDataTransfer(h, gsync)

	provider, err := impl.NewProvider(ds, bs, fs, piecestore.NewPieceStore(ds), dt, pnode)
	if err != nil {
		return nil, err
	}

	return &StorageProtocolSubmodule{
		StorageClient:   impl.NewClient(h, bs, fs, dt, nil, nil, cnode),
		StorageProvider: provider,
	}, nil
}
