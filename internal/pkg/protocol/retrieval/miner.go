package retrieval

import (
	"io/ioutil"

	logging "github.com/ipfs/go-log"
	host "github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	cbu "github.com/filecoin-project/go-filecoin/internal/pkg/cborutil"
	"github.com/filecoin-project/go-filecoin/internal/pkg/sectorbuilder"
)

var log = logging.Logger("/fil/retrieval")

const retrievalFreeProtocol = protocol.ID("/fil/retrieval/free/0.0.0")

// TODO: better name
type minerNode interface {
	Host() host.Host
	SectorBuilder() sectorbuilder.SectorBuilder
}

// Miner serves requests for pieces from RetrievalClients.
type Miner struct {
	node minerNode
}

// NewMiner is used to create a Miner and bind a handling function to the piece retrieval protocol.
func NewMiner(nd minerNode) *Miner {
	rm := &Miner{
		node: nd,
	}

	nd.Host().SetStreamHandler(retrievalFreeProtocol, rm.handleRetrievePieceForFree)

	return rm
}

func (rm *Miner) handleRetrievePieceForFree(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var req RetrievePieceRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&req); err != nil {
		log.Errorf("failed to read piece retrieval request: %s", err)
		return
	}

	reader, err := rm.node.SectorBuilder().ReadPieceFromSealedSector(req.PieceRef)
	if err != nil {
		log.Warnf("failed to obtain a reader for piece with CID %s: %s", req.PieceRef.String(), err)

		resp := RetrievePieceResponse{
			Status:       Failure,
			ErrorMessage: err.Error(),
		}

		if err := cbu.NewMsgWriter(s).WriteMsg(&resp); err != nil {
			log.Warnf("failed to write response for piece with CID %s: %s", req.PieceRef.String(), err)
		}

		return
	}

	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Errorf("failed to read all bytes: %s", err)
	}

	resp := RetrievePieceResponse{
		Status: Success,
	}

	if err := cbu.NewMsgWriter(s).WriteMsg(&resp); err != nil {
		log.Warnf("failed to write response for piece with CID %s: %s", req.PieceRef.String(), err)
		return
	}

	for i := 0; i < len(bs); i += RetrievePieceChunkSize {
		end := i + RetrievePieceChunkSize

		if end > len(bs) {
			end = len(bs)
		}

		chunk := RetrievePieceChunk{
			Data: bs[i:end],
		}

		if err := cbu.NewMsgWriter(s).WriteMsg(&chunk); err != nil {
			log.Warnf("failed to write chunk for CID %s: %s", req.PieceRef.String(), err)
			return
		}
	}
}
