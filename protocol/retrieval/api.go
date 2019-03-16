package retrieval

import (
	"context"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmTu65MVbemtUxJEWgsTtzv9Zv9P8rvmqNA4eG9TrTRGYc/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/address"
)

// API here is the API for a retrieval client.
type API struct {
	rc        *Client
	porcelain retrievalClientPorcelainAPI
}

type retrievalClientPorcelainAPI interface {
	MinerGetPeerID(ctx context.Context, minerAddr address.Address) (peer.ID, error)
}

// NewAPI creates a new API for a retrieval client.
func NewAPI(rc *Client, minerPorc retrievalClientPorcelainAPI) API {
	return API{rc: rc, porcelain: minerPorc}
}

// RetrievePiece retrieves bytes referenced by CID pieceCID
func (a *API) RetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error) {
	mpid, err := a.porcelain.MinerGetPeerID(ctx, minerAddr)
	if err != nil {
		return nil, err
	}
	return a.rc.RetrievePiece(ctx, mpid, pieceCID)
}
