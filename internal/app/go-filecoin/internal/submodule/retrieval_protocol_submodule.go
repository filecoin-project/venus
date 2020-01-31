package submodule

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/piecestore"
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
	impl "github.com/filecoin-project/go-fil-markets/retrievalmarket/impl"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/libp2p/go-libp2p-core/host"

	"github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/plumbing/msg"
	retrievalmarketconnector "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
	"github.com/filecoin-project/go-filecoin/internal/pkg/piecemanager"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// RetrievalProtocolSubmodule enhances the node with retrieval protocol
// capabilities.
type RetrievalProtocolSubmodule struct {
	RetrievalClient   iface.RetrievalClient
	RetrievalProvider iface.RetrievalProvider
}
type BalanceGetter func(ctx context.Context, address address.Address) (types.AttoFIL, error)
type WorkerGetter func(ctx context.Context, minerAddr address.Address, baseKey block.TipSetKey) (address.Address, error)

// NewRetrievalProtocolSubmodule creates a new retrieval protocol submodule.
func NewRetrievalProtocolSubmodule(
	ob *message.Outbox,
	smAPI piecemanager.StorageMinerAPI,
	host host.Host,
	providerAddr address.Address,
	c *ChainSubmodule,
	ps *piecestore.PieceStore,
	bs *blockstore.Blockstore) (RetrievalProtocolSubmodule, error) {
	panic("TODO: go-fil-markets integration")

	netwk := network.NewFromLibp2pHost(host)
	pnode := retrievalmarketconnector.NewRetrievalProviderNodeConnector(netwk, ps, bs)
	cnode := retrievalmarketconnector.NewRetrievalClientNodeConnector(
		bg, bs, c.ChainReader, ob, ps, smAPI, )
	rsvlr := retrievalmarketconnector.NewRetrievalPeerResolverConnector()

	return RetrievalProtocolSubmodule{
		RetrievalClient:   impl.NewClient(netwk, *bs, cnode, rsvlr),
		RetrievalProvider: impl.NewProvider(providerAddr, pnode, netwk, *ps, *bs),
	}, nil
}
