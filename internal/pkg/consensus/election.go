package consensus

import (
	"context"
	"encoding/binary"
	"math/big"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
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
func (em ElectionMachine) RunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullBlockCount uint64) (block.VRFPi, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	vrfPi, err := signer.SignBytes(buf[:], candidateAddr)
	if err != nil {
		return block.VRFPi{}, err
	}

	return block.VRFPi(vrfPi), nil
}

// IsElectionWinner verifies that an election proof was validly generated and
// is a winner.  TODO #3418 improve state management to clean up interface.
func (em ElectionMachine) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullBlockCount uint64, electionProof block.VRFPi, signingAddr, minerAddr address.Address) (bool, error) {
	seedBuf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(seedBuf, nullBlockCount)
	buf := append(ticket.VRFProof, seedBuf[:n]...)

	// Verify election proof is valid
	vrfPi := types.Signature(electionProof)
	if valid := types.IsValidSignature(buf[:], signingAddr, vrfPi); !valid {
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
// randomness function on the parent.
func (tm TicketMachine) NextTicket(parent block.Ticket, signerAddr address.Address, signer types.Signer) (block.Ticket, error) {
	vrfPi, err := signer.SignBytes(parent.VRFProof[:], signerAddr)
	if err != nil {
		return block.Ticket{}, err
	}

	return block.Ticket{
		VRFProof: block.VRFPi(vrfPi),
	}, nil
}

// IsValidTicket verifies that the ticket's proof of randomness and delay are
// valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	vrfPi := types.Signature(ticket.VRFProof)
	return types.IsValidSignature(parent.VRFProof[:], signerAddr, vrfPi)
}
