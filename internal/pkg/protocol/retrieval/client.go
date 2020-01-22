package retrieval

import (
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

// Client is...
type Client struct {
}

// NewClient is...
func NewClient() *Client {
	panic("replace with retrieval client in go-fil-markets")

	return &Client{} // nolint:govet
}

// RetrievePiece connects to a miner and transfers a piece of content.
func (sc *Client) RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID cid.Cid) (io.ReadCloser, error) {
	panic("replace with retrieval client in go-fil-markets")

	return nil, nil // nolint:govet
}
