package consensus

import (
	"context"
	"testing"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/ipfs/go-ipfs-blockstore"
	"github.com/ipfs/go-ipfs-exchange-offline"
	"github.com/stretchr/testify/require"

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

// TestPowerTableView is an implementation of the powertable view used for testing mining
// wherein each miner has totalPower/minerPower power.
type TestPowerTableView struct {
	minerPower, totalPower *types.BytesAmount
	minerToWorker          map[address.Address]address.Address
}

// NewTestPowerTableView creates a test power view with the given total power
func NewTestPowerTableView(minerPower *types.BytesAmount, totalPower *types.BytesAmount, minerToWorker map[address.Address]address.Address) *TestPowerTableView {
	return &TestPowerTableView{minerPower: minerPower, totalPower: totalPower, minerToWorker: minerToWorker}
}

// Total always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Total(ctx context.Context, st state.Tree, bstore blockstore.Blockstore) (*types.BytesAmount, error) {
	return tv.totalPower, nil
}

// Miner always returns value that was supplied to NewTestPowerTableView.
func (tv *TestPowerTableView) Miner(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) (*types.BytesAmount, error) {
	return tv.minerPower, nil
}

// HasPower always returns true.
func (tv *TestPowerTableView) HasPower(ctx context.Context, st state.Tree, bstore blockstore.Blockstore, mAddr address.Address) bool {
	return true
}

// WorkerAddr returns the address of the miner worker given the miner addr
func (tv *TestPowerTableView) WorkerAddr(ctx context.Context, st state.Tree, blk blockstore.Blockstore, minerAddr address.Address) (address.Address, error) {
	return tv.minerToWorker[minerAddr], nil
}

// FakeElectionMachine generates fake election proofs and verifies all proofs
type FakeElectionMachine struct{}

// RunElection returns a fake election proof.
func (fem *FakeElectionMachine) RunElection(ticket types.Ticket, candidateAddr address.Address, signer types.Signer) (types.VRFPi, error) {
	return MakeFakeElectionProofForTest(), nil
}

// IsElectionWinner always returns true
func (fem *FakeElectionMachine) IsElectionWinner(ctx context.Context, bs blockstore.Blockstore, ptv PowerTableView, st state.Tree, ticket types.Ticket, electionProof types.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
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
func (fev *FailingElectionValidator) IsElectionWinner(ctx context.Context, bs blockstore.Blockstore, ptv PowerTableView, st state.Tree, ticket types.Ticket, electionProof types.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
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

	// Lots of setup just to get an empty tree object :(
	// TODO #3078 should help with this
	mds := datastore.NewMapDatastore()
	bs := blockstore.NewBlockstore(mds)
	offl := offline.Exchange(bs)
	blkserv := blockservice.New(bs, offl)
	cst := &hamt.CborIpldStore{Blocks: blkserv}
	st := state.NewEmptyStateTree(cst)

	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)
	minerToWorker := make(map[address.Address]address.Address)
	minerToWorker[wAddr] = wAddr
	ptv := NewTestPowerTableView(types.NewBytesAmount(minerPower), types.NewBytesAmount(totalPower), minerToWorker)
	em := ElectionMachine{}
	tm := TicketMachine{}
	ctx := context.Background()

	curr := MakeFakeTicketForTest()
	tickets := []types.Ticket{curr}

	for {
		proof, err := em.RunElection(curr, wAddr, signer)
		require.NoError(t, err)

		wins, err := em.IsElectionWinner(ctx, bs, ptv, st, curr, proof, wAddr, wAddr)
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
