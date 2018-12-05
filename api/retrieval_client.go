package api

import (
	"context"
	"io"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/QmcqU6QUDSXprb1518vYDGczrTJTyGwLG9eUa5iNX4xUtS/go-libp2p-peer"
)

// RetrievalClient is the interface that defines methods to manage retrieval client operations.
type RetrievalClient interface {
	RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID cid.Cid) (io.ReadCloser, error)
}
