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

	retmkt "github.com/filecoin-project/go-filecoin/internal/app/go-filecoin/retrieval_market_connector"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/message"
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
	bs *blockstore.Blockstore,
	c *ChainSubmodule,
	mw retmkt.MsgWaiter,
	host host.Host,
	providerAddr address.Address,
	ob *message.Outbox,
	ps piecestore.PieceStore,
	signer types.Signer,
	wal retmkt.WalletAPI,
	aapi retmkt.ActorAPI,
	smapi retmkt.SmAPI,
	pbapi retmkt.PaymentBrokerAPI,
) (RetrievalProtocolSubmodule, error) {
	panic("TODO: go-fil-markets integration")

	netwk := network.NewFromLibp2pHost(host)
	pnode := retmkt.NewRetrievalProviderNodeConnector(netwk, ps, bs)
	cnode := retmkt.NewRetrievalClientNodeConnector(bs,
		c.ChainReader,
		mw,
		ob,
		ps,
		smapi,
		signer,
		aapi,
		wal,
		pbapi,
	)
	rsvlr := retmkt.NewRetrievalPeerResolverConnector()

	return RetrievalProtocolSubmodule{
		RetrievalClient:   impl.NewClient(netwk, *bs, cnode, rsvlr),
		RetrievalProvider: impl.NewProvider(providerAddr, pnode, netwk, ps, *bs),
	}, nil
}
