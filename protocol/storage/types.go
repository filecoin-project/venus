package storage

import (
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmRoARq3nkUb13HSKZGepCZSWe5GrVPwx7xURJGZ7KWv9V/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(DealProposal{})
	cbor.RegisterCborType(DealResponse{})
	cbor.RegisterCborType(ProofInfo{})
	cbor.RegisterCborType(queryRequest{})
}

// DealProposal is the information sent over the wire, when a client proposes a deal to a miner.
type DealProposal struct {
	// PieceRef is the cid of the piece being stored
	PieceRef cid.Cid

	// Size is the total number of bytes the proposal is asking to store
	Size *types.BytesAmount

	// TotalPrice is the total price that will be paid for the entire storage operation
	TotalPrice *types.AttoFIL

	// Duration is the number of blocks to make a deal for
	Duration uint64

	// TODO: Payment PaymentInfo
	// Signature types.Signature
}

// DealResponse is the information sent over the wire, when a miner responds to a client.
type DealResponse struct {
	// State is the current state of this deal
	State DealState

	// Message is an optional message to add context to any given response
	Message string

	// Proposal is the cid of the StorageDealProposal object this response is for
	Proposal cid.Cid

	// ProofInfo is a collection of information needed to convince the client that
	// the miner has sealed the data into a sector.
	ProofInfo *ProofInfo

	// Signature is a signature from the miner over the response
	Signature types.Signature
}

// ProofInfo contains the details about a seal proof, that the client needs to know to verify that his deal was posted on chain.
// TODO: finalize parameters
type ProofInfo struct {
	SectorID uint64
	CommR    []byte
	CommD    []byte
}

type queryRequest struct {
	Cid cid.Cid
}
