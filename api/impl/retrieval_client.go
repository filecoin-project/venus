package impl

import (
	"context"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
)

type nodeRetrievalClient struct {
	api *nodeAPI
}

func newNodeRetrievalClient(api *nodeAPI) *nodeRetrievalClient {
	return &nodeRetrievalClient{api: api}
}

func (nrc *nodeRetrievalClient) RetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error) {
	minerPeerID, err := nrc.api.node.Lookup().GetPeerIDByMinerAddress(ctx, minerAddr)
	if err != nil {
		return nil, err
	}

	return nrc.api.node.RetrievalClient.RetrievePiece(ctx, minerPeerID, pieceCID)
}
