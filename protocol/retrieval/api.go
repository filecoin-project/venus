package retrieval

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

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
