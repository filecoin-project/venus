package consensus

import (
	"bytes"
	"context"
	"math/big"

	"github.com/filecoin-project/go-filecoin/block"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// CompareElectionPower return true if the input electionProof is below the
// election victory threshold for the input miner and global power values.
func CompareElectionPower(electionProof block.VRFPi, minerPower *types.BytesAmount, totalPower *types.BytesAmount) bool {
	lhs := &big.Int{}
	lhs.SetBytes(electionProof)
	lhs.Mul(lhs, totalPower.BigInt())
	rhs := &big.Int{}
	rhs.Mul(minerPower.BigInt(), ticketDomain)

	return lhs.Cmp(rhs) < 0
}

// ElectionMachine generates and validates election proofs from tickets.
type ElectionMachine struct{}

// RunElection uses a VRF to run a secret, verifiable election with respect to
// an input ticket.
func (em ElectionMachine) RunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer) (block.VRFPi, error) {
	vrfPi, err := signer.SignBytes(ticket.VDFResult[:], candidateAddr)
	if err != nil {
		return block.VRFPi{}, err
	}

	return block.VRFPi(vrfPi), nil
}

// IsElectionWinner verifies that an election proof was validly generated and
// is a winner.  TODO #3418 improve state management to clean up interface.
func (em ElectionMachine) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, electionProof block.VRFPi, signingAddr, minerAddr address.Address) (bool, error) {
	// Verify election proof is valid
	vrfPi := types.Signature(electionProof)
	if valid := types.IsValidSignature(ticket.VDFResult, signingAddr, vrfPi); !valid {
		return false, nil
	}

	// Verify election proof is a winner
	totalPower, err := ptv.Total(ctx)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get totalPower")
	}

	minerPower, err := ptv.Miner(ctx, minerAddr)
	if err != nil {
		return false, errors.Wrap(err, "Couldn't get minerPower")
	}

	return CompareElectionPower(electionProof, minerPower, totalPower), nil
}

// TicketMachine uses a VRF and VDF to generate deterministic, unpredictable
// and time delayed tickets and validates these tickets.
type TicketMachine struct{}

// NextTicket creates a new ticket from a parent ticket by running a verifiable
// randomness function on the VDF output of the parent.  The output ticket is
// not finalized until it has been run through NotarizeTime.
func (tm TicketMachine) NextTicket(parent block.Ticket, signerAddr address.Address, signer types.Signer) (block.Ticket, error) {
	vrfPi, err := signer.SignBytes(parent.VDFResult[:], signerAddr)
	if err != nil {
		return block.Ticket{}, err
	}

	return block.Ticket{
		VRFProof: block.VRFPi(vrfPi),
	}, nil
}

// NotarizeTime finalizes the input ticket by running a verifiable delay
// function on the input ticket's VRFProof.  It updates the VDFResult and
// VDFProof of the input ticket.
func (tm TicketMachine) NotarizeTime(ticket *block.Ticket) error {
	// TODO #2119, decide if we are going to keep the VDF.  If so fill
	// in the implementation by actually generationg the VDF output and
	// proof.
	ticket.VDFResult = block.VDFY(ticket.VRFProof[:])
	return nil
}

// IsValidTicket verifies that the ticket's proof of randomness and delay are
// valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	vrfPi := types.Signature(ticket.VRFProof)
	if valid := types.IsValidSignature(parent.VDFResult[:], signerAddr, vrfPi); !valid {
		return false
	}

	// TODO #2119, decide if we are going to keep the VDF.  If so fill
	// in the implementation, removing this equality check and actually
	// validating the VDFProof.
	return bytes.Equal(ticket.VDFResult, ticket.VRFProof)
}
