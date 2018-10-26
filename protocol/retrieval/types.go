package retrieval

import (
	cbor "gx/ipfs/QmV6BQ6fFCf9eFHDuRxvguvqfKLZtZrxthgZvDfRCs4tMN/go-ipld-cbor"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

func init() {
	cbor.RegisterCborType(RetrievePieceRequest{})
	cbor.RegisterCborType(RetrievePieceResponse{})
	cbor.RegisterCborType(RetrievePieceChunk{})
}

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

// RetrievePieceRequest represents a retrieval miner's request for content.
type RetrievePieceRequest struct {
	PieceRef *cid.Cid
}

// RetrievePieceResponse contains the requested content.
type RetrievePieceResponse struct {
	Status       RetrievePieceStatus
	ErrorMessage string
}

// RetrievePieceChunk is a subset of bytes for a piece being retrieved.
type RetrievePieceChunk struct {
	Data []byte
}
