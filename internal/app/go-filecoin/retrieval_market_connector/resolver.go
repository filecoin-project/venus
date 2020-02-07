package retrievalmarketconnector

import (
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"
)

type RetrievalPeerResolverConnector struct{}

// var _ retrievalmarket.PeerResolver = &RetrievalPeerResolverConnector{}

func NewRetrievalPeerResolverConnector() *RetrievalPeerResolverConnector {
	return &RetrievalPeerResolverConnector{}
}

func (r RetrievalPeerResolverConnector) GetPeers(payloadCID cid.Cid) ([]retrievalmarket.RetrievalPeer, error) {
	panic("implement me")
}


