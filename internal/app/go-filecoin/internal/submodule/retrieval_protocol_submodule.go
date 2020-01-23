package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalClient   iface.RetrievalClient
	RetrievalProvider iface.RetrievalProvider
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(providerAddr address.Address, rs iface.PeerResolver, nt rmnet.RetrievalMarketNetwork, ps piecestore.PieceStore, bs blockstore.Blockstore) (RetrievalProtocolSubmodule, error) {
	pnode := retrievalmarketconnector.NewRetrievalProviderNodeConnector()
	cnode := retrievalmarketconnector.NewRetrievalClientNodeConnector()

	return RetrievalProtocolSubmodule{
		RetrievalClient:   impl.NewClient(nt, bs, cnode, rs),
		RetrievalProvider: impl.NewProvider(providerAddr, pnode, nt, ps, bs),
	}, nil
}
