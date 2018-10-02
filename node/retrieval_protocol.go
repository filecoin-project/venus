package node

import (
	"io/ioutil"

	inet "gx/ipfs/QmQSbtGXCyNrj34LWL8EgXyNNYDZ8r3SwQcpW5pPxVhLnM/go-libp2p-net"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
)

const RetrievePieceForFreeProtocolID = protocol.ID("/fil/retrieval/free/0.0.0") // nolint: golint

// RetrievalMiner serves requests for pieces from RetrievalClients.
type RetrievalMiner struct {
	nd *Node
}

// NewRetrievalMiner is used to create a RetrievalMiner and bind a handling function to the piece retrieval protocol.
func NewRetrievalMiner(nd *Node) *RetrievalMiner {
	rm := &RetrievalMiner{
		nd: nd,
	}

	nd.Host.SetStreamHandler(RetrievePieceForFreeProtocolID, rm.handleRetrievePieceForFree)

	return rm
}

func (rm *RetrievalMiner) handleRetrievePieceForFree(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var req RetrievePieceRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&req); err != nil {
		log.Errorf("failed to read piece retrieval request: %s", err)
		return
	}

	reader, err := rm.nd.SectorBuilder.ReadPieceFromSealedSector(req.PieceRef)
	if err != nil {
		log.Warningf("failed to obtain a reader for piece with CID %s: %s", req.PieceRef.String(), err)

		resp := RetrievePieceResponse{
			Status:       Failure,
			ErrorMessage: err.Error(),
		}

		if err := cbu.NewMsgWriter(s).WriteMsg(&resp); err != nil {
			log.Warningf("failed to write response for piece with CID %s: %s", req.PieceRef.String(), err)
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
		log.Warningf("failed to write response for piece with CID %s: %s", req.PieceRef.String(), err)
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
			log.Warningf("failed to write chunk for CID %s: %s", req.PieceRef.String(), err)
			return
		}
	}
}
