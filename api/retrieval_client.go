package api

import (
	"context"
	"io"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

// RetrievalClient is the interface that defines methods to manage retrieval client operations.
type RetrievalClient interface {
	RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID *cid.Cid) (io.ReadCloser, error)
}
