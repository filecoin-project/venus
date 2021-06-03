package consensus

import (
	"bytes"
	"context"

	"github.com/filecoin-project/venus/pkg/chain"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	acrypto "github.com/filecoin-project/go-state-types/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/pkg/errors"

	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/types"
)

type ChainSampler interface {
	SampleTicket(ctx context.Context, head types.TipSetKey, epoch abi.ChainEpoch, lookback bool) (types.Ticket, error)
}

type tipsetLoader interface {
	GetTipSet(types.TipSetKey) (*types.TipSet, error)
}

// TicketMachine uses a VRF and VDF to generate deterministic, unpredictable
// and time delayed tickets and validates these tickets.
type TicketMachine struct {
	sampler      ChainSampler
	tipsetLoader tipsetLoader
}

func NewTicketMachine(sampler ChainSampler, tipsetLoader tipsetLoader) *TicketMachine {
	return &TicketMachine{sampler: sampler, tipsetLoader: tipsetLoader}
}

// MakeTicket creates a new ticket from a Chain and target epoch by running a verifiable
// randomness function on the prior ticket.
func (tm TicketMachine) MakeTicket(ctx context.Context, base types.TipSetKey, epoch abi.ChainEpoch, miner address.Address, entry *types.BeaconEntry, newPeriod bool, worker address.Address, signer types.Signer) (types.Ticket, error) {
	randomness, err := tm.ticketVRFRandomness(ctx, base, entry, newPeriod, miner, epoch)
	if err != nil {
		return types.Ticket{}, errors.Wrap(err, "failed to generate ticket randomness")
	}
	vrfProof, err := signer.SignBytes(ctx, randomness, worker)
	if err != nil {
		return types.Ticket{}, errors.Wrap(err, "failed to sign election post randomness")
	}
	return types.Ticket{
		VRFProof: vrfProof.Data,
	}, nil
}

// IsValidTicket verifies that the ticket's proof of randomness is valid with respect to its parent.
func (tm TicketMachine) IsValidTicket(ctx context.Context, base types.TipSetKey, entry *types.BeaconEntry, bSmokeHeight bool,
	epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket types.Ticket) error {
	randomness, err := tm.ticketVRFRandomness(ctx, base, entry, bSmokeHeight, miner, epoch)
	if err != nil {
		return errors.Wrap(err, "failed to generate ticket randomness")
	}

	return crypto.ValidateBlsSignature(randomness, workerSigner, ticket.VRFProof)
}

func (tm TicketMachine) ticketVRFRandomness(ctx context.Context, base types.TipSetKey, entry *types.BeaconEntry, bSmokeHeight bool, miner address.Address, epoch abi.ChainEpoch) (abi.Randomness, error) {
	entropyBuf := new(bytes.Buffer)
	err := miner.MarshalCBOR(entropyBuf)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to encode miner entropy")
	}

	if bSmokeHeight { // todo
		ts, err := tm.tipsetLoader.GetTipSet(base)
		if err != nil {
			return nil, err
		}
		_, err = entropyBuf.Write(ts.MinTicket().VRFProof)
		if err != nil {
			return nil, err
		}
	}
	seed := blake2b.Sum256(entry.Data)
	return chain.BlendEntropy(acrypto.DomainSeparationTag_TicketProduction, seed[:], epoch, entropyBuf.Bytes())
}
