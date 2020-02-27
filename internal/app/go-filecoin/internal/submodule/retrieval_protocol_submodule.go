package submodule

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/cst"
	retmkt "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	pc *retmkt.RetrievalProviderConnector
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
	pnode := retmkt.NewRetrievalProviderConnector(netwk, pieceManager, bs, pchMgrAPI)

	// TODO: use latest go-fil-markets with persisted deal store
	marketProvider := impl.NewProvider(providerAddr, pnode, netwk, retrievalDealPieceStore, bs)

	pnode.SetProvider(marketProvider)

	return &RetrievalProtocolSubmodule{pnode}, nil
}
