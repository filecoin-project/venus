package node

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"

	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
)

func init() {
	cbor.RegisterCborType(RetrievePieceRequest{})
	cbor.RegisterCborType(RetrievePieceResponse{})
	cbor.RegisterCborType(RetrievePieceChunk{})
}

// RetrievePieceChunkSize defines the size of piece-chunks to be sent from miner to client. The maximum size of readable
// message is defined as cborutil.MaxMessageSize. Our chunk size needs to be less than that value in order for reads to
// succeed.
const RetrievePieceChunkSize = 256 << 8

// RetrievePieceStatus communicates a successful (or failed) piece retrieval
type RetrievePieceStatus int

const (
	// Unset is the default status
	Unset = RetrievePieceStatus(iota)

	// Failure indicates that the piece could not be retrieved from the miner
	Failure

	// Success means that the piece could be retrieved from the miner
	Success
)

// RetrievePieceResponse contains the requested content.
type RetrievePieceResponse struct {
	Status       RetrievePieceStatus
	ErrorMessage string
}

// RetrievePieceChunk is a subset of bytes for a piece being retrieved.
type RetrievePieceChunk struct {
	Data []byte
}

// RetrievePieceRequest represents a retrieval miner's request for content.
type RetrievePieceRequest struct {
	PieceRef *cid.Cid
}

// RetrievalClient is a client interface to the retrieval market protocols.
type RetrievalClient struct {
	nd *Node
}

// NewRetrievalClient produces a new RetrievalClient.
func NewRetrievalClient(nd *Node) *RetrievalClient {
	return &RetrievalClient{
		nd: nd,
	}
}

// RetrievePiece connects to a miner and transfers a piece of content.
func (sc *RetrievalClient) RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID *cid.Cid) (io.ReadCloser, error) {
	s, err := sc.nd.Host.NewStream(ctx, minerPeerID, RetrievePieceForFreeProtocolID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream to retrieval miner")
	}

	defer s.Close() // nolint: errcheck

	streamReader := cbu.NewMsgReader(s)

	req := RetrievePieceRequest{
		PieceRef: pieceCID,
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(&req); err != nil {
		return nil, errors.Wrap(err, "failed to write request message to stream")
	}

	var res RetrievePieceResponse
	if err := streamReader.ReadMsg(&res); err != nil {
		return nil, errors.Wrap(err, "failed to read response message from stream")
	}

	if res.Status != Success {
		return nil, errors.Errorf("could not retrieve piece - error from miner: %s", res.ErrorMessage)
	}

	var buf []byte
	for {
		var chunk RetrievePieceChunk
		if err := streamReader.ReadMsg(&chunk); err != nil {
			if err == io.EOF {
				break
			}

			return nil, errors.Errorf("could not read chunk from stream: %s", err.Error())
		}

		buf = append(buf, chunk.Data...)
	}

	// TODO: Figure out how to stream piece-bytes w/out having to buffer.
	buffered := ioutil.NopCloser(bytes.NewReader(buf))

	return buffered, nil
}
