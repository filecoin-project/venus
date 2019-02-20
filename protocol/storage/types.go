package storage

import (
	"github.com/filecoin-project/go-filecoin/actor/builtin/paymentbroker"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	cbor "gx/ipfs/QmcZLyosDwMKdB6NLRsiss9HXzDPhVhhRtPy67JFKTDQDX/go-ipld-cbor"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

func init() {
	cbor.RegisterCborType(PaymentInfo{})
	cbor.RegisterCborType(DealProposal{})
	cbor.RegisterCborType(DealResponse{})
	cbor.RegisterCborType(ProofInfo{})
	cbor.RegisterCborType(queryRequest{})
}

// PaymentInfo contains all the payment related information for a storage deal.
type PaymentInfo struct {
	// PayChActor is the address of the payment channel actor
	// that will be used to facilitate payments
	PayChActor address.Address

	// Payer is the address of the owner of the payment channel
	Payer address.Address

	// Channel is the ID of the specific channel the client will
	// use to pay the miner. It must already have sufficient funds locked up
	Channel *types.ChannelID

	// ChannelMsgCid is the B58 encoded CID of the message used to create the channel (so the miner can wait for it).
	ChannelMsgCid *cid.Cid

	// Vouchers is a set of payments from the client to the miner that can be
	// cashed out contingent on the agreed upon data being provably within a
	// live sector in the miners control on-chain
	Vouchers []*paymentbroker.PaymentVoucher
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

	// MinerAddress is the address of the storage miner in the deal proposal
	MinerAddress address.Address

	// Payment is a reference to the mechanism that the proposer
	// will use to pay the miner. It should be verifiable by the
	// miner using on-chain information.
	Payment PaymentInfo

	// Signature types.Signature
}

// DealResponse is the information sent over the wire, when a miner responds to a client.
type DealResponse struct {
	// State is the current state of this deal
	State DealState

	// Message is an optional message to add context to any given response
	Message string

	// Proposal is the cid of the StorageDealProposal object this response is for
	ProposalCid cid.Cid

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
