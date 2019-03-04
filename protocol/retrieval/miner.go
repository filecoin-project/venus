package retrieval

import (
	"io/ioutil"

	inet "gx/ipfs/QmTGxDz2CjBucFzPNTiWwzQmTWdrBnzqbqrMucDYMsjuPb/go-libp2p-net"
	"gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	logging "gx/ipfs/QmbkT7eMTyXfpeyB3ZMxxcxg7XH8t6uXp49jqzz4HB7BGF/go-log"

	cbu "github.com/filecoin-project/go-filecoin/cborutil"
	"github.com/filecoin-project/go-filecoin/node/sectorforeman"
)

var log = logging.Logger("/fil/retrieval")

const retrievalFreeProtocol = protocol.ID("/fil/retrieval/free/0.0.0")

// TODO: better name
type minerPlumbing interface {
	NetworkSetStreamHandler(protocol.ID, inet.StreamHandler)
}

// Miner serves requests for pieces from RetrievalClients.
type Miner struct {
	plumbing      minerPlumbing
	sectorForeman *sectorforeman.SectorForeman
}

// NewMiner is used to create a Miner and bind a handling function to the piece retrieval protocol.
func NewMiner(plumbing minerPlumbing, sectorForeman *sectorforeman.SectorForeman) *Miner {
	rm := &Miner{
		plumbing:      plumbing,
		sectorForeman: sectorForeman,
	}

	plumbing.NetworkSetStreamHandler(retrievalFreeProtocol, rm.handleRetrievePieceForFree)

	return rm
}

func (rm *Miner) handleRetrievePieceForFree(s inet.Stream) {
	defer s.Close() // nolint: errcheck

	var req RetrievePieceRequest
	if err := cbu.NewMsgReader(s).ReadMsg(&req); err != nil {
		log.Errorf("failed to read piece retrieval request: %s", err)
		return
	}

	reader, err := rm.sectorForeman.ReadPieceFromSealedSector(req.PieceRef)
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
