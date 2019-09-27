package consensus

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/state"
	"github.com/filecoin-project/go-filecoin/types"
)

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*types.Block) types.TipSet {
	ts, err := types.NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// FakeActorStateStore provides a snapshot that responds to power table view queries with the given parameters
type FakeActorStateStore struct {
	minerPower    *types.BytesAmount
	totalPower    *types.BytesAmount
	minerToWorker map[address.Address]address.Address
}

// NewFakeActorStateStore creates an actor state that returns a test queryer such that PowerTableView queries return predefined results
func NewFakeActorStateStore(minerPower, totalPower *types.BytesAmount, minerToWorker map[address.Address]address.Address) *FakeActorStateStore {
	return &FakeActorStateStore{
		minerPower:    minerPower,
		totalPower:    totalPower,
		minerToWorker: minerToWorker,
	}
}

// StateTreeSnapshot returns a Snapshot suitable for PowerTableView queries
func (t *FakeActorStateStore) StateTreeSnapshot(st state.Tree, bh *types.BlockHeight) ActorStateSnapshot {
	return &FakePowerTableViewSnapshot{
		MinerPower:    t.minerPower,
		TotalPower:    t.totalPower,
		MinerToWorker: t.minerToWorker,
	}
}

// FakePowerTableViewSnapshot returns a queryer that can be fed into a PowerTableView to produce specific values
type FakePowerTableViewSnapshot struct {
	MinerPower    *types.BytesAmount
	TotalPower    *types.BytesAmount
	MinerToWorker map[address.Address]address.Address
}

// Query produces test logic in response to PowerTableView queries.
func (tq *FakePowerTableViewSnapshot) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	if method == "getTotalStorage" {
		if tq.TotalPower != nil {
			return [][]byte{tq.TotalPower.Bytes()}, nil
		}
		return [][]byte{}, errors.New("something went wrong with the total power")
	} else if method == "getPower" {
		if tq.MinerPower != nil {
			return [][]byte{tq.MinerPower.Bytes()}, nil
		}
		return [][]byte{}, errors.New("something went wrong with the miner power")
	} else if method == "getWorker" {
		if tq.MinerToWorker != nil {
			return [][]byte{tq.MinerToWorker[to].Bytes()}, nil
		}
	}
	return [][]byte{}, fmt.Errorf("unknown method for TestQueryer '%s'", method)
}

// NewFakePowerTableView creates a test power view with the given total power
func NewFakePowerTableView(minerPower *types.BytesAmount, totalPower *types.BytesAmount, minerToWorker map[address.Address]address.Address) PowerTableView {
	tq := &FakePowerTableViewSnapshot{
		MinerPower:    minerPower,
		TotalPower:    totalPower,
		MinerToWorker: minerToWorker,
	}
	return NewPowerTableView(tq)
}

// FakeSignedMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type FakeSignedMessageValidator struct{}

var _ SignedMessageValidator = (*FakeSignedMessageValidator)(nil)

// Validate always returns nil
func (tsmv *FakeSignedMessageValidator) Validate(ctx context.Context, msg *types.SignedMessage, fromActor *actor.Actor) error {
	return nil
}

// FakeBlockRewarder is a rewarder that doesn't actually add any rewards to simplify state tracking in tests
type FakeBlockRewarder struct{}

var _ BlockRewarder = (*FakeBlockRewarder)(nil)

// BlockReward is a noop
func (tbr *FakeBlockRewarder) BlockReward(ctx context.Context, st state.Tree, minerAddr address.Address) error {
	// do nothing to keep state root the same
	return nil
}

// GasReward is a noop
func (tbr *FakeBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerAddr address.Address, msg *types.SignedMessage, gas types.AttoFIL) error {
	// do nothing to keep state root the same
	return nil
}

// NewFakeProcessor creates a processor with a test validator and test rewarder
func NewFakeProcessor() *DefaultProcessor {
	return &DefaultProcessor{
		signedMessageValidator: &FakeSignedMessageValidator{},
		blockRewarder:          &FakeBlockRewarder{},
	}
}

// FakeElectionMachine generates fake election proofs and verifies all proofs
type FakeElectionMachine struct{}

// RunElection returns a fake election proof.
func (fem *FakeElectionMachine) RunElection(ticket types.Ticket, candidateAddr address.Address, signer types.Signer) (types.VRFPi, error) {
	return MakeFakeElectionProofForTest(), nil
}

// IsElectionWinner always returns true
func (fem *FakeElectionMachine) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket types.Ticket, electionProof types.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	return true, nil
}

// FakeTicketMachine generates fake tickets and verifies all tickets
type FakeTicketMachine struct{}

// NextTicket returns a fake ticket
func (ftm *FakeTicketMachine) NextTicket(parent types.Ticket, signerAddr address.Address, signer types.Signer) (types.Ticket, error) {
	return MakeFakeTicketForTest(), nil
}

// NotarizeTime does nothing
func (ftm *FakeTicketMachine) NotarizeTime(ticket *types.Ticket) error {
	return nil
}

// IsValidTicket always returns true
func (ftm *FakeTicketMachine) IsValidTicket(parent, ticket types.Ticket, signerAddr address.Address) bool {
	return true
}

// FailingTicketValidator marks all tickets as invalid
type FailingTicketValidator struct{}

// IsValidTicket always returns false
func (ftv *FailingTicketValidator) IsValidTicket(parent, ticket types.Ticket, signerAddr address.Address) bool {
	return false
}

// FailingElectionValidator marks all elections as invalid
type FailingElectionValidator struct{}

// IsElectionWinner always returns false
func (fev *FailingElectionValidator) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket types.Ticket, electionProof types.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	return false, nil
}

// MakeFakeTicketForTest creates a fake ticket
func MakeFakeTicketForTest() types.Ticket {
	val := make([]byte, 65)
	val[0] = 200
	return types.Ticket{
		VRFProof:  types.VRFPi(val[:]),
		VDFResult: types.VDFY(val[:]),
	}
}

// MakeFakeElectionProofForTest creates a fake election proof
func MakeFakeElectionProofForTest() []byte {
	proof := make([]byte, 65)
	proof[0] = 42
	return proof
}

// SeedFirstWinnerInNRounds returns a ticket that when mined upon for N rounds
// by a miner that has `minerPower` out of a system-wide `totalPower` and keyinfo
// `ki` will produce a ticket that gives a winning election proof in exactly `n`
// rounds.  There are no winning tickets in between the seed and the Nth ticket.
//
// Note that this is a deterministic function of the inputs as we return the
// first ticket that seeds a winner in `n` rounds given the inputs starting from
// MakeFakeTicketForTest().
//
// Note that there are no guarantees that this function will terminate on new
// inputs as miner power might be so low that winning a ticket is very
// unlikely.  However runtime is deterministic so if it runs fast once on
// given inputs is safe to use in tests.
func SeedFirstWinnerInNRounds(t *testing.T, n int, ki *types.KeyInfo, minerPower, totalPower uint64) types.Ticket {
	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[wAddr] = wAddr
	ptv := NewFakePowerTableView(types.NewBytesAmount(minerPower), types.NewBytesAmount(totalPower), minerToWorker)
	em := ElectionMachine{}
	tm := TicketMachine{}
	ctx := context.Background()

	curr := MakeFakeTicketForTest()
	tickets := []types.Ticket{curr}

	for {
		proof, err := em.RunElection(curr, wAddr, signer)
		require.NoError(t, err)

		wins, err := em.IsElectionWinner(ctx, ptv, curr, proof, wAddr, wAddr)
		require.NoError(t, err)
		if wins {
			// We have enough tickets, we're done
			if len(tickets) >= n+1 {
				return tickets[len(tickets)-1-n]
			}

			// We won too early, reset memory
			tickets = []types.Ticket{}
		}

		// make a new ticket off the chain
		curr, err = tm.NextTicket(curr, wAddr, signer)
		require.NoError(t, err)
		require.NoError(t, tm.NotarizeTime(&curr))

		tickets = append(tickets, curr)
	}

}
