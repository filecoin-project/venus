package submodule

import (
	"context"
	"os"

	"github.com/ipfs/go-graphsync"

	"github.com/filecoin-project/go-address"
	graphsyncimpl "github.com/filecoin-project/go-data-transfer/impl/graphsync"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/storagemarket"
	impl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	smnetwork "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	storagemarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/storage_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/wallet"
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
	minerAddr address.Address,
	clientAddr address.Address,
	c *ChainSubmodule,
	m *MessagingSubmodule,
	mw *msg.Waiter,
	pm piecemanager.PieceManager,
	wlt *wallet.Wallet,
	h host.Host,
	ds datastore.Batching,
	bs blockstore.Blockstore,
	gsync graphsync.GraphExchange,
	repoPath string,
	wg storagemarketconnector.WorkerGetter,
	sealProofType abi.RegisteredProof,

) (*StorageProtocolSubmodule, error) {

	pnode := storagemarketconnector.NewStorageProviderNodeConnector(minerAddr, c.State, m.Outbox, mw, pm, wg, wlt)
	cnode := storagemarketconnector.NewStorageClientNodeConnector(cborutil.NewIpldStore(bs), c.State, mw, wlt, m.Outbox, clientAddr, wg)

	pieceStagingPath, err := paths.PieceStagingDir(repoPath)
	if err != nil {
		return nil, err
	}

	// ensure pieces directory exists
	err = os.MkdirAll(pieceStagingPath, 0700)
	if err != nil {
		return nil, err
	}

	fs, err := filestore.NewLocalFileStore(filestore.OsPath(pieceStagingPath))
	if err != nil {
		return nil, err
	}

	dt := graphsyncimpl.NewGraphSyncDataTransfer(h, gsync)

	provider, err := impl.NewProvider(smnetwork.NewFromLibp2pHost(h), ds, bs, fs, piecestore.NewPieceStore(ds), dt, pnode, minerAddr, sealProofType)
	if err != nil {
		return nil, errors.Wrap(err, "error creating graphsync provider")
	}

	return &StorageProtocolSubmodule{
		StorageClient:   impl.NewClient(smnetwork.NewFromLibp2pHost(h), bs, dt, nil, nil, cnode),
		StorageProvider: provider,
	}, nil
}
