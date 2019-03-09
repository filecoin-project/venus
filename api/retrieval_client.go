package api

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/address"
)

// RetrievalClient is the interface that defines methods to manage retrieval client operations.
type RetrievalClient interface {
	RetrievePiece(ctx context.Context, pieceCID cid.Cid, minerAddr address.Address) (io.ReadCloser, error)
}
