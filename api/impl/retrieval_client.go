package impl

import (
	"context"
	"io"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

type nodeRetrievalClient struct {
	api *nodeAPI
}

func newNodeRetrievalClient(api *nodeAPI) *nodeRetrievalClient {
	return &nodeRetrievalClient{api: api}
}

func (nrc *nodeRetrievalClient) RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID *cid.Cid) (io.ReadCloser, error) {
	return nrc.api.node.RetrievalClient.RetrievePiece(ctx, minerPeerID, pieceCID)
}
