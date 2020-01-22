package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	rmnet "github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalClient *retrieval.Client
	RetrievalMiner *retrieval.Provider
}

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(providerAddr address.Address, nd retrievalmarket.RetrievalProviderNode, nt rmnet.RetrievalMarketNetwork, ps piecestore.PieceStore, bs blockstore.Blockstore) (RetrievalProtocolSubmodule, error) {
	return RetrievalProtocolSubmodule{
		// RetrievalAPI: nil,
		RetrievalMiner: retrieval.NewProvider(providerAddr, nd, nt, ps, bs),
	}, nil
}
