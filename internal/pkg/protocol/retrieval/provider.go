package retrieval

import (
	a2 "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	r "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
)

// Provider is...
type Provider struct {
}

// NewProvider consummates retrieval deals.
func NewProvider(pa a2.Address, nd retrievalmarket.RetrievalProviderNode, nt rmnet.RetrievalMarketNetwork, ps piecestore.PieceStore, bs blockstore.Blockstore) *Provider {
	r.NewProvider(pa, nd, nt, ps, bs)

	return &Provider{}
}
