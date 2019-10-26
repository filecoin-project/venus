package retrieval

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"

	cbu "github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/net"
)

// RetrievePieceChunkSize defines the size of piece-chunks to be sent from miner to client. The maximum size of readable
// message is defined as cborutil.MaxMessageSize. Our chunk size needs to be less than that value in order for reads to
// succeed.
const RetrievePieceChunkSize = 256 << 8

type clientPorcelainAPI interface {
	PingMinerWithTimeout(ctx context.Context, p peer.ID, to time.Duration) error
}

// Client is a client interface to the retrieval market protocols.
type Client struct {
	api  clientPorcelainAPI
	host host.Host
	log  logging.EventLogger
}

// NewClient produces a new Client.
func NewClient(host host.Host, api clientPorcelainAPI) *Client {
	return &Client{
		api:  api,
		host: host,
		log:  logging.Logger("retrieval/client"),
	}
}

// RetrievePiece connects to a miner and transfers a piece of content.
func (sc *Client) RetrievePiece(ctx context.Context, minerPeerID peer.ID, pieceCID cid.Cid) (io.ReadCloser, error) {
	err := sc.api.PingMinerWithTimeout(ctx, minerPeerID, 15*time.Second)
	if err == net.ErrPingSelf {
		return nil, errors.New("attempting to retrieve piece from self. This is currently unsupported.  Please use a separate go-filecoin node as client")
	}
	if err != nil {
		return nil, err
	}
	s, err := sc.host.NewStream(ctx, minerPeerID, retrievalFreeProtocol)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream to retrieval miner")
	}
	defer sc.safeCloseStream(s)

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

func (sc *Client) safeCloseStream(stream inet.Stream) {
	if err := stream.Close(); err != nil {
		log.Errorf("error closing stream: %s", err)
	}
}
