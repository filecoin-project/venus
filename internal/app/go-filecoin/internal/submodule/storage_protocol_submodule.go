package submodule

import (
	"context"
	"os"

	"github.com/filecoin-project/go-address"
	graphsyncimpl "github.com/filecoin-project/go-data-transfer/impl/graphsync"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/storagemarket"
	impl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	smnetwork "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-graphsync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/pkg/errors"

	storagemarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/storage_market"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/paths"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	"github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// StorageProtocolSubmodule enhances the node with storage protocol
// capabilities.
type StorageProtocolSubmodule struct {
	StorageClient   iface.StorageClient
	StorageProvider iface.StorageProvider
	pieceManager    piecemanager.PieceManager
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(
	ctx context.Context,
	clientAddr address.Address,
	c *ChainSubmodule,
	m *MessagingSubmodule,
	mw *msg.Waiter,
	s types.Signer,
	h host.Host,
	ds datastore.Batching,
	bs blockstore.Blockstore,
	gsync graphsync.GraphExchange,
	stateViewer *appstate.Viewer,
) (*StorageProtocolSubmodule, error) {
	cnode := storagemarketconnector.NewStorageClientNodeConnector(cborutil.NewIpldStore(bs), c.State, mw, s, m.Outbox, clientAddr, stateViewer)

	dt := graphsyncimpl.NewGraphSyncDataTransfer(h, gsync)

	client, err := impl.NewClient(smnetwork.NewFromLibp2pHost(h), bs, dt, nil, ds, cnode)
	if err != nil {
		return nil, errors.Wrap(err, "error creating storage client")
	}

	return &StorageProtocolSubmodule{
		StorageClient: client,
	}, nil
}

func (sm *StorageProtocolSubmodule) AddStorageProvider(
	ctx context.Context,
	minerAddr address.Address,
	c *ChainSubmodule,
	m *MessagingSubmodule,
	mw *msg.Waiter,
	pm piecemanager.PieceManager,
	s types.Signer,
	h host.Host,
	ds datastore.Batching,
	bs blockstore.Blockstore,
	gsync graphsync.GraphExchange,
	repoPath string,
	sealProofType abi.RegisteredProof,
	stateViewer *appstate.Viewer,
) error {
	sm.pieceManager = pm

	pnode := storagemarketconnector.NewStorageProviderNodeConnector(minerAddr, c.State, m.Outbox, mw, pm, s, stateViewer)

	pieceStagingPath, err := paths.PieceStagingDir(repoPath)
	if err != nil {
		return err
	}

	// ensure pieces directory exists
	err = os.MkdirAll(pieceStagingPath, 0700)
	if err != nil {
		return err
	}

	fs, err := filestore.NewLocalFileStore(filestore.OsPath(pieceStagingPath))
	if err != nil {
		return err
	}

	dt := graphsyncimpl.NewGraphSyncDataTransfer(h, gsync)

	sm.StorageProvider, err = impl.NewProvider(smnetwork.NewFromLibp2pHost(h), ds, bs, fs, piecestore.NewPieceStore(ds), dt, pnode, minerAddr, sealProofType)
	return err
}

func (sm *StorageProtocolSubmodule) Provider() iface.StorageProvider {
	return sm.StorageProvider
}

func (sm *StorageProtocolSubmodule) Client() iface.StorageClient {
	return sm.StorageClient
}

func (sm *StorageProtocolSubmodule) PieceManager() piecemanager.PieceManager {
	return sm.pieceManager
}
