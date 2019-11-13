package consensus

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/power"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/state"
)

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*block.Block) block.TipSet {
	ts, err := block.NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// FakeActorStateStore provides a snapshot that responds to power table view queries with the given parameters
type FakeActorStateStore struct {
	minerPower    *types.BytesAmount
	totalPower    *types.BytesAmount
	minerToWorker map[address.Address]address.Address
}

// NewFakeActorStateStore creates an actor state store that produces snapshots such that PowerTableView queries return predefined results
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

// FakePowerTableViewSnapshot returns a snapshot that can be fed into a PowerTableView to produce specific values
type FakePowerTableViewSnapshot struct {
	MinerPower    *types.BytesAmount
	TotalPower    *types.BytesAmount
	MinerToWorker map[address.Address]address.Address
}

// Query produces test logic in response to PowerTableView queries.
func (tq *FakePowerTableViewSnapshot) Query(ctx context.Context, optFrom, to address.Address, method types.MethodID, params ...interface{}) ([][]byte, error) {
	// Note: this currently happens to work as is, but it's wrong
	// Note: a better mock is recommended to make sure the correct methods get dispatched
	if method == power.GetTotalPower {
		if tq.TotalPower != nil {
			return [][]byte{tq.TotalPower.Bytes()}, nil
		}
		return [][]byte{}, errors.New("something went wrong with the total power")
	} else if method == power.GetPowerReport {
		if tq.MinerPower != nil {
			powerReport := types.NewPowerReport(tq.MinerPower.Uint64(), 0)
			val := abi.Value{
				Val:  powerReport,
				Type: abi.PowerReport,
			}
			raw, err := val.Serialize()
			return [][]byte{raw}, err
		}
		return [][]byte{}, errors.New("something went wrong with the miner power")
	} else if method == miner.GetWorker {
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

// FakeMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type FakeMessageValidator struct{}

// Validate always returns nil
func (tsmv *FakeMessageValidator) Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error {
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
func (tbr *FakeBlockRewarder) GasReward(ctx context.Context, st state.Tree, minerOwnerAddr address.Address, msg *types.UnsignedMessage, cost types.AttoFIL) error {
	// do nothing to keep state root the same
	return nil
}

// NewFakeProcessor creates a processor with a test validator and test rewarder
func NewFakeProcessor(actors builtin.Actors) *DefaultProcessor {
	return &DefaultProcessor{
		validator:     &FakeMessageValidator{},
		blockRewarder: &FakeBlockRewarder{},
		actors:        actors,
	}
}

// FakeElectionMachine generates fake election proofs and verifies all proofs
type FakeElectionMachine struct{}

// RunElection returns a fake election proof.
func (fem *FakeElectionMachine) RunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullCount uint64) (block.VRFPi, error) {
	return MakeFakeElectionProofForTest(), nil
}

// IsElectionWinner always returns true
func (fem *FakeElectionMachine) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullCount uint64, electionProof block.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	return true, nil
}

// FakeTicketMachine generates fake tickets and verifies all tickets
type FakeTicketMachine struct{}

// NextTicket returns a fake ticket
func (ftm *FakeTicketMachine) NextTicket(parent block.Ticket, signerAddr address.Address, signer types.Signer) (block.Ticket, error) {
	return MakeFakeTicketForTest(), nil
}

// IsValidTicket always returns true
func (ftm *FakeTicketMachine) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	return true
}

// FailingTicketValidator marks all tickets as invalid
type FailingTicketValidator struct{}

// IsValidTicket always returns false
func (ftv *FailingTicketValidator) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	return false
}

// FailingElectionValidator marks all elections as invalid
type FailingElectionValidator struct{}

// IsElectionWinner always returns false
func (fev *FailingElectionValidator) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullCount uint64, electionProof block.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	return false, nil
}

// MakeFakeTicketForTest creates a fake ticket
func MakeFakeTicketForTest() block.Ticket {
	val := make([]byte, 65)
	val[0] = 200
	return block.Ticket{
		VRFProof: block.VRFPi(val[:]),
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
// rounds.  No wins before n rounds are up.
//
// Note that this is a deterministic function of the inputs as we return the
// first ticket that seeds a winner in `n` rounds given the inputs starting from
// MakeFakeTicketForTest().
//
// Note that there are no guarantees that this function will terminate on new
// inputs as miner power might be so low that winning a ticket is very
// unlikely.  However runtime is deterministic so if it runs fast once on
// given inputs is safe to use in tests.
func SeedFirstWinnerInNRounds(t *testing.T, n int, ki *types.KeyInfo, minerPower, totalPower uint64) block.Ticket {
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

	for {
		// does it win at n rounds?
		proof, err := em.RunElection(curr, wAddr, signer, uint64(n))
		require.NoError(t, err)

		wins, err := em.IsElectionWinner(ctx, ptv, curr, uint64(n), proof, wAddr, wAddr)
		require.NoError(t, err)
		if wins {
			// does it have no wins before n rounds?
			if losesAllRounds(t, n-1, curr, wAddr, signer, ptv, em) {
				return curr
			}
		}

		// make a new ticket off the previous
		curr, err = tm.NextTicket(curr, wAddr, signer)
		require.NoError(t, err)
	}
}

func losesAllRounds(t *testing.T, n int, ticket block.Ticket, wAddr address.Address, signer types.Signer, ptv PowerTableView, em ElectionMachine) bool {
	for i := 0; i < n; i++ {
		losesAtRound(t, i, ticket, wAddr, signer, ptv, em)

	}
	return true
}

func losesAtRound(t *testing.T, n int, ticket block.Ticket, wAddr address.Address, signer types.Signer, ptv PowerTableView, em ElectionMachine) bool {
	proof, err := em.RunElection(ticket, wAddr, signer, uint64(n))
	require.NoError(t, err)

	wins, err := em.IsElectionWinner(context.Background(), ptv, ticket, uint64(n), proof, wAddr, wAddr)
	require.NoError(t, err)
	return !wins
}

// SeedLoserInNRounds returns a ticket that loses with a null block count of N.
func SeedLoserInNRounds(t *testing.T, n int, ki *types.KeyInfo, minerPower, totalPower uint64) block.Ticket {
	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[wAddr] = wAddr
	ptv := NewFakePowerTableView(types.NewBytesAmount(minerPower), types.NewBytesAmount(totalPower), minerToWorker)
	em := ElectionMachine{}
	tm := TicketMachine{}

	curr := MakeFakeTicketForTest()

	for {
		if losesAtRound(t, n, curr, wAddr, signer, ptv, em) {
			return curr
		}

		// make a new ticket off the previous
		curr, err = tm.NextTicket(curr, wAddr, signer)
		require.NoError(t, err)
	}
}

// MockTicketMachine allows a test to set a function to be called upon ticket
// generation and validation
type MockTicketMachine struct {
	fn func(block.Ticket)
}

// NewMockTicketMachine creates a mock given a callback
func NewMockTicketMachine(f func(block.Ticket)) *MockTicketMachine {
	return &MockTicketMachine{fn: f}
}

// NextTicket calls the registered callback and returns a fake ticket
func (mtm *MockTicketMachine) NextTicket(ticket block.Ticket, genAddr address.Address, signer types.Signer) (block.Ticket, error) {
	mtm.fn(ticket)
	return MakeFakeTicketForTest(), nil
}

// IsValidTicket calls the registered callback and returns true
func (mtm *MockTicketMachine) IsValidTicket(parent, ticket block.Ticket, signerAddr address.Address) bool {
	mtm.fn(ticket)
	return true
}

// MockElectionMachine allows a test to set a function to be called upon
// election running and validation
type MockElectionMachine struct {
	fn func(block.Ticket)
}

// NewMockElectionMachine creates a mock given a callback
func NewMockElectionMachine(f func(block.Ticket)) *MockElectionMachine {
	return &MockElectionMachine{fn: f}
}

// RunElection calls the registered callback and returns a fake proof
func (mem *MockElectionMachine) RunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullCount uint64) (block.VRFPi, error) {
	mem.fn(ticket)
	return MakeFakeElectionProofForTest(), nil
}

// IsElectionWinner calls the registered callback and returns true
func (mem *MockElectionMachine) IsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullCount uint64, electionProof block.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	mem.fn(ticket)
	return true, nil
}
