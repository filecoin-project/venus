package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/discovery"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/filecoin-project/go-fil-markets/storedcounter"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"

	retmkt "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/connectors/retrieval_market"
	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	pc *retmkt.RetrievalProviderConnector
	cc *retmkt.RetrievalClientConnector
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(
	bs blockstore.Blockstore,
	ds datastore.Batching,
	cr *cst.ChainStateReadWriter,
	host host.Host,
	providerAddr address.Address,
	signer retmkt.RetrievalSigner,
	pchMgrAPI retmkt.PaychMgrAPI,
	pieceManager piecemanager.PieceManager,
) (*RetrievalProtocolSubmodule, error) {

	retrievalDealPieceStore := piecestore.NewPieceStore(ds)

	netwk := network.NewFromLibp2pHost(host)
	pnode := retmkt.NewRetrievalProviderConnector(netwk, pieceManager, bs, pchMgrAPI, nil)

	// TODO: use latest go-fil-markets with persisted deal store
	marketProvider, err := impl.NewProvider(providerAddr, pnode, netwk, retrievalDealPieceStore, bs, ds)
	if err != nil {
		return nil, err
	}
	pnode.SetProvider(marketProvider)

	cnode := retmkt.NewRetrievalClientConnector(bs, cr, signer, pchMgrAPI)
	dsKey := datastore.NewKey("retrievalmarket/client/counter")
	counter := storedcounter.New(ds, dsKey)
	resolver := discovery.Multi(discovery.NewLocal(ds))
	marketClient, err := impl.NewClient(netwk, bs, cnode, resolver, ds, counter)
	if err != nil {
		return nil, err
	}
	cnode.SetRetrievalClient(marketClient)

	return &RetrievalProtocolSubmodule{pnode, cnode}, nil
}
