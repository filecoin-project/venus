package retrievalmarketconnector

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"
)

// RetrievalPeerResolverConnector is glue code between go-filecoin and the peer resolver api from
// go-fil-markets
type RetrievalPeerResolverConnector struct{}

// var _ retrievalmarket.PeerResolver = &RetrievalPeerResolverConnector{}

// NewRetrievalPeerResolverConnector creates a new RetrievalPeerResolverConnector
func NewRetrievalPeerResolverConnector() *RetrievalPeerResolverConnector {
	return &RetrievalPeerResolverConnector{}
}

// GetPeers gets all the peers storing piece with payloadCID
func (r RetrievalPeerResolverConnector) GetPeers(payloadCID cid.Cid) ([]retrievalmarket.RetrievalPeer, error) {
	panic("implement me")
}
