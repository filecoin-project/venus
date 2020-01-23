package retrievalmarketconnector

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type RetrievalMarketNetworkConnector struct{}

func NewRetrievalMarketNetworkConnector() *RetrievalMarketNetworkConnector {
	return &RetrievalMarketNetworkConnector{}
}

func (n *RetrievalMarketNetworkConnector) NewQueryStream(peerID peer.ID) (network.RetrievalQueryStream, error) {
	panic("TODO: go-fil-markets integration")
}

func (n *RetrievalMarketNetworkConnector) NewDealStream(peerID peer.ID) (network.RetrievalDealStream, error) {
	panic("TODO: go-fil-markets integration")
}

func (n *RetrievalMarketNetworkConnector) SetDelegate(network.RetrievalReceiver) error {
	panic("TODO: go-fil-markets integration")
}
