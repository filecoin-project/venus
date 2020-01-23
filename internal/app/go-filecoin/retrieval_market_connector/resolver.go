package retrievalmarketconnector

import (
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

type RetrievalPeerResolverConnector struct{}

func NewRetrievalPeerResolverConnector() *RetrievalPeerResolverConnector {
	return &RetrievalPeerResolverConnector{}
}

func (r *RetrievalPeerResolverConnector) GetPeers(pieceCID []byte) ([]iface.RetrievalPeer, error) {
	panic("TODO: go-fil-markets integration")
}
