package storagedeal

import (
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

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
	Vouchers []*types.PaymentVoucher
}

// Proposal is the information sent over the wire, when a client proposes a deal to a miner.
type Proposal struct {
	// PieceRef is the cid of the piece being stored
	PieceRef cid.Cid

	// Size is the total number of bytes the proposal is asking to store
	Size *types.BytesAmount

	// TotalPrice is the total price that will be paid for the entire storage operation
	TotalPrice types.AttoFIL

	// Duration is the number of blocks to make a deal for
	Duration uint64

	// MinerAddress is the address of the storage miner in the deal proposal
	MinerAddress address.Address

	// Payment is a reference to the mechanism that the proposer
	// will use to pay the miner. It should be verifiable by the
	// miner using on-chain information.
	Payment PaymentInfo
}

// Unmarshal a Proposal from bytes.
func (dp *Proposal) Unmarshal(b []byte) error {
	return encoding.Decode(b, dp)
}

// Marshal the Proposal into bytes.
func (dp *Proposal) Marshal() ([]byte, error) {
	return encoding.Encode(dp)
}

// NewSignedProposal signs Proposal with address `addr` and returns a SignedProposal.
func (dp *Proposal) NewSignedProposal(addr address.Address, signer types.Signer) (*SignedProposal, error) {
	data, err := dp.Marshal()
	if err != nil {
		return nil, err
	}

	sig, err := signer.SignBytes(data, addr)
	if err != nil {
		return nil, err
	}
	return &SignedProposal{
		Proposal:  *dp,
		Signature: sig,
	}, nil
}

// SignedProposal is a deal proposal signed by the proposing client
type SignedProposal struct {
	Proposal
	// Signature is the signature of the client proposing the deal.
	Signature types.Signature
}

// Response is the information sent over the wire, when a miner responds to a client.
type Response struct {
	// State is the current state of this deal
	State State

	// Message is an optional message to add context to any given response
	Message string

	// Proposal is the cid of the StorageDealProposal object this response is for
	ProposalCid cid.Cid

	// ProofInfo is a collection of information needed to convince the client that
	// the miner has sealed the data into a sector.
	ProofInfo *ProofInfo
}

// SignedResponse is a signed wrapper around response
type SignedResponse struct {
	Response

	// Signature is a signature from the miner over the response
	Signature types.Signature
}

// Sign signs this response
func (r *SignedResponse) Sign(signer types.Signer, addr address.Address) error {
	respBytes, err := encoding.Encode(r.Response)
	if err != nil {
		return err
	}

	r.Signature, err = signer.SignBytes(respBytes, addr)
	return err
}

// VerifySignature verifies the signature of this response
func (r *SignedResponse) VerifySignature(addr address.Address) (bool, error) {
	respBytes, err := encoding.Encode(r.Response)
	if err != nil {
		return false, err
	}

	return types.IsValidSignature(respBytes, addr, r.Signature), nil
}

// Deal is a storage deal struct
type Deal struct {
	Miner    address.Address
	CommP    types.CommP
	Proposal *SignedProposal
	Response *SignedResponse
}

// ProofInfo contains the details about a seal proof, that the client needs to know to verify that his deal was posted on chain.
type ProofInfo struct {
	// Sector id allows us to find the committed sector metadata on chain
	SectorID uint64

	// CommD of the sector
	CommD []byte

	// CommR of the replica
	CommR []byte

	// CommRStar of the replica
	CommRStar []byte

	// CommitmentMessage is the cid of the message that committed the sector. It's used to track when the sector goes on chain.
	CommitmentMessage cid.Cid

	// PieceInclusionProof is a proof that a the piece is included within a sector
	PieceInclusionProof []byte
}

// QueryRequest is used for making protocol api requests for deals
type QueryRequest struct {
	Cid cid.Cid
}
