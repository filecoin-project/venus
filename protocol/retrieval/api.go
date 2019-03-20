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
	rc *Client
}

// NewAPI creates a new API for a retrieval client.
func NewAPI(rc *Client) API {
	return API{rc: rc}
}

// RetrievePiece retrieves bytes referenced by CID pieceCID
func (a *API) RetrievePiece(ctx context.Context, pieceCID cid.Cid, mpid peer.ID, minerAddr address.Address) (io.ReadCloser, error) {
	return a.rc.RetrievePiece(ctx, mpid, pieceCID)
}
