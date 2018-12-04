package impl

import (
	"context"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
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
