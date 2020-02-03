package retrievalmarketconnector

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

// RetrievalMarketNetworkConnector adapts the node for usage by the retrieval market
type RetrievalMarketNetworkConnector struct{}

// NewRetrievalMarketNetworkConnector creates a new connector
func NewRetrievalMarketNetworkConnector() *RetrievalMarketNetworkConnector {
	return &RetrievalMarketNetworkConnector{}
}

// NewQueryStream creates a new retrieval query stream
func (n *RetrievalMarketNetworkConnector) NewQueryStream(peerID peer.ID) (network.RetrievalQueryStream, error) {
	panic("TODO: go-fil-markets integration")
}

// NewDealStream creates a new deal stream
func (n *RetrievalMarketNetworkConnector) NewDealStream(peerID peer.ID) (network.RetrievalDealStream, error) {
	panic("TODO: go-fil-markets integration")
}

// SetDelegate sets the retrieval receiver
func (n *RetrievalMarketNetworkConnector) SetDelegate(network.RetrievalReceiver) error {
	panic("TODO: go-fil-markets integration")
}
