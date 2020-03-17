package retrievalmarketconnector

import (
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

// RetrievalPeerResolverConnector adapts the node to provide an interface for the RetrievalPeer
type RetrievalPeerResolverConnector struct{}

// NewRetrievalPeerResolverConnector creates a new connector
func NewRetrievalPeerResolverConnector() *RetrievalPeerResolverConnector {
	return &RetrievalPeerResolverConnector{}
}

// GetPeers gets peers for the piece CID
func (r *RetrievalPeerResolverConnector) GetPeers(pieceCID []byte) ([]iface.RetrievalPeer, error) {
	panic("TODO: go-fil-markets integration")
}
