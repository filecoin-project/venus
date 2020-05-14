package submodule

import (
	"context"
	"os"

	"github.com/filecoin-project/go-statestore"
	"github.com/filecoin-project/go-storedcounter"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	graphsyncimpl "github.com/filecoin-project/go-data-transfer/impl/graphsync"
	"github.com/filecoin-project/go-fil-markets/filestore"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	iface "github.com/filecoin-project/go-fil-markets/storagemarket"
	impl "github.com/filecoin-project/go-fil-markets/storagemarket/impl"
	smvalid "github.com/filecoin-project/go-fil-markets/storagemarket/impl/requestvalidation"
	smnetwork "github.com/filecoin-project/go-fil-markets/storagemarket/network"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-graphsync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	logging "github.com/ipfs/go-log"
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

var storageLog = logging.Logger("storage-protocol")

// StorageProtocolSubmodule enhances the node with storage protocol
// capabilities.
type StorageProtocolSubmodule struct {
	StorageClient    iface.StorageClient
	StorageProvider  iface.StorageProvider
	dataTransfer     datatransfer.Manager
	requestValidator *smvalid.UnifiedRequestValidator
	pieceManager     piecemanager.PieceManager
}

// NewStorageProtocolSubmodule creates a new storage protocol submodule.
func NewStorageProtocolSubmodule(
	ctx context.Context,
	clientAddr storagemarketconnector.ClientAddressGetter,
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
	dtStoredCounter := storedcounter.New(ds, datastore.NewKey("datatransfer/counter"))
	dt := graphsyncimpl.NewGraphSyncDataTransfer(h, gsync, dtStoredCounter)
	clientDs := namespace.Wrap(ds, datastore.NewKey("/deals/client"))
	validator := smvalid.NewUnifiedRequestValidator(nil, statestore.New(clientDs))
	err := dt.RegisterVoucherType(&smvalid.StorageDataTransferVoucher{}, validator)
	if err != nil {
		return nil, err
	}

	client, err := impl.NewClient(smnetwork.NewFromLibp2pHost(h), bs, dt, discovery.NewLocal(ds), clientDs, cnode)
	if err != nil {
		return nil, errors.Wrap(err, "error creating storage client")
	}

	sm := &StorageProtocolSubmodule{
		StorageClient:    client,
		dataTransfer:     dt,
		requestValidator: validator,
	}
	sm.StorageClient.SubscribeToEvents(sm.clientEventLogger)
	return sm, nil
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

	providerDs := namespace.Wrap(ds, datastore.NewKey(impl.ProviderDsPrefix))
	sm.requestValidator.SetPushDeals(statestore.New(providerDs))
	// TODO: see https://github.com/filecoin-project/go-fil-markets/issues/251 -- this should accept providerDs so
	// the node can configure the namespace
	sm.StorageProvider, err = impl.NewProvider(smnetwork.NewFromLibp2pHost(h), ds, bs, fs, piecestore.NewPieceStore(ds), sm.dataTransfer, pnode, minerAddr, sealProofType)
	if err == nil {
		sm.StorageProvider.SubscribeToEvents(sm.providerEventLogger)
	}
	return err
}

func (sm *StorageProtocolSubmodule) Provider() (iface.StorageProvider, error) {
	if sm.StorageProvider == nil {
		return nil, errors.New("Mining has not been started so storage provider is not available")
	}
	return sm.StorageProvider, nil
}

func (sm *StorageProtocolSubmodule) Client() iface.StorageClient {
	return sm.StorageClient
}

func (sm *StorageProtocolSubmodule) PieceManager() (piecemanager.PieceManager, error) {
	if sm.StorageProvider == nil {
		return nil, errors.New("Mining has not been started so piece manager is not available")
	}
	return sm.pieceManager, nil
}

func (sm *StorageProtocolSubmodule) DataTransfer() datatransfer.Manager {
	return sm.dataTransfer
}

func (sm *StorageProtocolSubmodule) RequestValidator() datatransfer.RequestValidator {
	return sm.requestValidator
}

func (sm *StorageProtocolSubmodule) clientEventLogger(event iface.ClientEvent, deal iface.ClientDeal) {
	storageLog.Infof("Event: %s, Proposal CID: %s, State: %s, Message: %s", iface.ClientEvents[event], deal.ProposalCid, iface.DealStates[deal.State], deal.Message)
}

func (sm *StorageProtocolSubmodule) providerEventLogger(event iface.ProviderEvent, deal iface.MinerDeal) {
	storageLog.Infof("Event: %s, Proposal CID: %s, State: %s, Message: %s", iface.ProviderEvents[event], deal.ProposalCid, iface.DealStates[deal.State], deal.Message)
}
