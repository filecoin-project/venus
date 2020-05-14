package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-storedcounter"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"
	xerrors "github.com/pkg/errors"

	retmkt "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	client   iface.RetrievalClient
	provider iface.RetrievalProvider
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(bs blockstore.Blockstore,
	ds datastore.Batching,
	cr *cst.ChainStateReadWriter,
	h host.Host,
	signer retmkt.RetrievalSigner,
	pchMgrAPI retmkt.PaychMgrAPI,
) (*RetrievalProtocolSubmodule, error) {
	netwk := network.NewFromLibp2pHost(h)
	cnode := retmkt.NewRetrievalClientConnector(bs, cr, signer, pchMgrAPI)
	dsKey := datastore.NewKey("retrievalmarket/client/counter")
	counter := storedcounter.New(ds, dsKey)
	resolver := discovery.Multi(discovery.NewLocal(ds))
	marketClient, err := impl.NewClient(netwk, bs, cnode, resolver, ds, counter)
	if err != nil {
		return nil, err
	}
	log.Infof("marketClient: %v", marketClient)
	return &RetrievalProtocolSubmodule{client: marketClient}, nil
}

func (rps *RetrievalProtocolSubmodule) Client() iface.RetrievalClient {
	return rps.client
}

func (rps *RetrievalProtocolSubmodule) Provider() (iface.RetrievalProvider, error) {
	if !rps.IsProvider() {
		return nil, xerrors.New("retrieval provider not configured")
	}
	return rps.provider, nil
}

func (rps *RetrievalProtocolSubmodule) IsProvider() bool {
	return rps.provider != nil
}

func (rps *RetrievalProtocolSubmodule) AddProvider(h host.Host,
	providerAddr address.Address,
	pm piecemanager.PieceManager,
	bs blockstore.Blockstore,
	ds datastore.Batching,
	pchMgrAPI retmkt.PaychMgrAPI) error {
	netwk := network.NewFromLibp2pHost(h)
	pnode := retmkt.NewRetrievalProviderConnector(netwk, pm, bs, pchMgrAPI, nil)
	retrievalDealPieceStore := piecestore.NewPieceStore(ds)

	// TODO: use latest go-fil-markets with persisted deal store
	marketProvider, err := impl.NewProvider(providerAddr, pnode, netwk, retrievalDealPieceStore, bs, ds)
	if err != nil {
		return err
	}
	rps.provider = marketProvider
	return nil
}
