package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalClient   iface.RetrievalClient
	RetrievalProvider iface.RetrievalProvider
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(providerAddr address.Address, ps piecestore.PieceStore, bs blockstore.Blockstore) (*RetrievalProtocolSubmodule, error) {
	panic("TODO: go-fil-markets integration")

	//pnode := retrievalmarketconnector.NewRetrievalProviderNodeConnector()
	//cnode := retrievalmarketconnector.NewRetrievalClientNodeConnector()
	//netwk := retrievalmarketconnector.NewRetrievalMarketNetworkConnector()
	//rsvlr := retrievalmarketconnector.NewRetrievalPeerResolverConnector()
	//
	//return &RetrievalProtocolSubmodule{
	//	RetrievalClient:   impl.NewClient(netwk, bs, cnode, rsvlr),
	//	RetrievalProvider: impl.NewProvider(providerAddr, pnode, netwk, ps, bs),
	//}, nil
}
