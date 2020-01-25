package retrievalmarketconnector

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket/network"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
)

type RetrievalMarketNetworkConnector struct{
	net network.RetrievalMarketNetwork
}

func NewRetrievalMarketNetworkConnector(host host.Host, peerID peer.ID) *RetrievalMarketNetworkConnector {
	net := network.NewFromLibp2pHost(host)
	connector := RetrievalMarketNetworkConnector{net}
	return &connector
}

func (n *RetrievalMarketNetworkConnector) NewQueryStream(peerID peer.ID) (network.RetrievalQueryStream, error) {
	return n.net.NewQueryStream(peerID)
}

func (n *RetrievalMarketNetworkConnector) NewDealStream(peerID peer.ID) (network.RetrievalDealStream, error) {
	return n.net.NewDealStream(peerID)
}

func (n *RetrievalMarketNetworkConnector) SetDelegate(r network.RetrievalReceiver) error {
	return n.net.SetDelegate(r)
}
