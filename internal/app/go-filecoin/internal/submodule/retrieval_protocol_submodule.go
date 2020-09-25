package submodule

import (
	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	appstate "github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-multistore"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"

	retmkt "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// RetrievalProviderDSPrefix is a prefix for all datastore keys related to the retrieval provider
const RetrievalProviderDSPrefix = "/retrievalmarket/provider"

// RetrievalCounterDSKey is the datastore key for the stored counter used by the retrieval counter
const RetrievalCounterDSKey = "/retrievalmarket/client/counter"

// RetrievalClientDSPrefix is a prefix for all datastore keys related to the retrieval clients
const RetrievalClientDSPrefix = "/retrievalmarket/client"

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	client   iface.RetrievalClient
	provider iface.RetrievalProvider
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(
	bs blockstore.Blockstore,
	ds datastore.Batching,
	mds *multistore.MultiStore,
	cr *cst.ChainStateReadWriter,
	host host.Host,
	providerAddr address.Address,
	signer retmkt.RetrievalSigner,
	pchMgrAPI retmkt.PaychMgrAPI,
	pieceManager piecemanager.PieceManager,
	dtTransfer datatransfer.Manager,
	viewer *appstate.TipSetStateViewer,
) (*RetrievalProtocolSubmodule, error) {

	retrievalDealPieceStore := piecestore.NewPieceStore(namespace.Wrap(ds, datastore.NewKey(PieceStoreDSPrefix)))

	netwk := network.NewFromLibp2pHost(host)
	pnode := retmkt.NewRetrievalProviderConnector(netwk, pieceManager, bs, pchMgrAPI, nil)

	marketProvider, err := impl.NewProvider(providerAddr, pnode, netwk, retrievalDealPieceStore, mds, dtTransfer, namespace.Wrap(ds, datastore.NewKey(RetrievalProviderDSPrefix)))
	if err != nil {
		return nil, err
	}

	cnode := retmkt.NewRetrievalClientConnector(bs, cr, signer, pchMgrAPI, viewer)
	counter := storedcounter.New(ds, datastore.NewKey(RetrievalCounterDSKey))

	resolver := discovery.Multi(discovery.NewLocal(namespace.Wrap(ds, datastore.NewKey(DiscoveryDSPrefix))))
	marketClient, err := impl.NewClient(netwk, mds, dtTransfer, cnode, resolver, namespace.Wrap(ds, datastore.NewKey(RetrievalClientDSPrefix)), counter)
	if err != nil {
		return nil, err
	}

	return &RetrievalProtocolSubmodule{marketClient, marketProvider}, nil
}

func (rps *RetrievalProtocolSubmodule) Client() iface.RetrievalClient {
	return rps.client
}

func (rps *RetrievalProtocolSubmodule) Provider() iface.RetrievalProvider {
	return rps.provider
}
