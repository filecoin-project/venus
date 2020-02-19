package consensus

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs"
	"github.com/filecoin-project/go-filecoin/internal/pkg/proofs/verification"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/util/hasher"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*block.Block) block.TipSet {
	ts, err := block.NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// FakePowerStateViewer is a fake power state viewer.
type FakePowerStateViewer struct {
	Views map[cid.Cid]*state.FakeStateView
}

// StateView returns the state view for a root.
func (f *FakePowerStateViewer) StateView(root cid.Cid) PowerStateView {
	return f.Views[root]
}

// FakeMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type FakeMessageValidator struct{}

// Validate always returns nil
func (tsmv *FakeMessageValidator) Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error {
	return nil
}

// NewFakeProcessor creates a processor with a test validator and test rewarder
func NewFakeProcessor(actors vm.ActorCodeLoader) *DefaultProcessor {
	return &DefaultProcessor{
		actors: actors,
	}
}

// FakeElectionMachine generates fake election proofs and verifies all proofs
type FakeElectionMachine struct{}

var _ ElectionValidator = new(FakeElectionMachine)

// DeprecatedRunElection returns a fake election proof.
func (fem *FakeElectionMachine) DeprecatedRunElection(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullCount uint64) (block.VRFPi, error) {
	return MakeFakeVRFProofForTest(), nil
}

// DeprecatedIsElectionWinner always returns true
func (fem *FakeElectionMachine) DeprecatedIsElectionWinner(ctx context.Context, ptv PowerTableView, ticket block.Ticket, nullCount uint64, electionProof block.VRFPi, signerAddr, minerAddr address.Address) (bool, error) {
	return true, nil
}

// GeneratePoStRandomness returns a fake post randomness byte array
func (fem *FakeElectionMachine) GeneratePoStRandomness(_ block.Ticket, _ address.Address, _ types.Signer, _ uint64) ([]byte, error) {
	return MakeFakeVRFProofForTest(), nil
}

// GenerateCandidates returns one fake election post candidate
func (fem *FakeElectionMachine) GenerateCandidates(_ []byte, _ ffi.SortedPublicSectorInfo, _ postgenerator.PoStGenerator) ([]ffi.Candidate, error) {
	return []ffi.Candidate{
		{
			SectorNum:            0,
			PartialTicket:        [32]byte{0xf},
			Ticket:               [32]byte{0xe},
			SectorChallengeIndex: 0,
		},
	}, nil
}

// GeneratePoSt returns a fake post proof
func (fem *FakeElectionMachine) GeneratePoSt(_ ffi.SortedPublicSectorInfo, _ []byte, _ []ffi.Candidate, _ postgenerator.PoStGenerator) ([]byte, error) {
	return MakeFakePoStForTest(), nil
}

// VerifyPoStRandomness returns true
func (fem *FakeElectionMachine) VerifyPoStRandomness(_ block.VRFPi, _ block.Ticket, _ address.Address, _ uint64) bool {
	return true
}

// CandidateWins returns true
func (fem *FakeElectionMachine) CandidateWins(_ []byte, _ uint64, _ uint64, _ uint64, _ uint64) bool {
	return true
}

// VerifyPoSt return true
func (fem *FakeElectionMachine) VerifyPoSt(_ verification.PoStVerifier, _ ffi.SortedPublicSectorInfo, _ uint64, _ []byte, _ []byte, _ []block.EPoStCandidate, _ address.Address) (bool, error) {
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

// FailingElectionValidator marks all election candidates as invalid
type FailingElectionValidator struct{}

var _ ElectionValidator = new(FailingElectionValidator)

// CandidateWins always returns false
func (fev *FailingElectionValidator) CandidateWins(_ []byte, _, _, _, _ uint64) bool {
	return false
}

// VerifyPoSt returns true without error
func (fev *FailingElectionValidator) VerifyPoSt(_ verification.PoStVerifier, _ ffi.SortedPublicSectorInfo, _ uint64, _ []byte, _ []byte, _ []block.EPoStCandidate, _ address.Address) (bool, error) {
	return true, nil
}

// VerifyPoStRandomness return true
func (fev *FailingElectionValidator) VerifyPoStRandomness(_ block.VRFPi, _ block.Ticket, _ address.Address, _ uint64) bool {
	return true
}

// MakeFakeTicketForTest creates a fake ticket
func MakeFakeTicketForTest() block.Ticket {
	val := make([]byte, 65)
	val[0] = 200
	return block.Ticket{
		VRFProof: block.VRFPi(val[:]),
	}
}

// MakeFakeVRFProofForTest creates a fake election proof
func MakeFakeVRFProofForTest() []byte {
	proof := make([]byte, 65)
	proof[0] = 42
	return proof
}

// MakeFakePoStForTest creates a fake post
func MakeFakePoStForTest() []byte {
	proof := make([]byte, 1)
	proof[0] = 0xe
	return proof
}

// MakeFakeWinnersForTest creats an empty winners array
func MakeFakeWinnersForTest() []block.EPoStCandidate {
	return []block.EPoStCandidate{}
}

// NFakeSectorInfos returns numSectors fake sector infos
func NFakeSectorInfos(numSectors uint64) ffi.SortedPublicSectorInfo {
	var infos []ffi.PublicSectorInfo
	for i := uint64(0); i < numSectors; i++ {
		buf := make([]byte, binary.MaxVarintLen64)
		binary.PutUvarint(buf, i)
		var fakeCommRi [ffi.CommitmentBytesLen]byte
		copy(fakeCommRi[:], buf)
		infos = append(infos, ffi.PublicSectorInfo{
			SectorNum: abi.SectorNumber(i),
			CommR:     fakeCommRi,
		})
	}

	return ffi.NewSortedPublicSectorInfo(infos...)
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
func SeedFirstWinnerInNRounds(t *testing.T, n int, ki *types.KeyInfo, networkPower, numSectors, sectorSize uint64) block.Ticket {
	tm := TicketMachine{}
	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)

	// give it some fake sector infos
	sectorInfos := NFakeSectorInfos(numSectors)
	curr := MakeFakeTicketForTest()

	for {
		if winsAtEpoch(t, uint64(n), curr, ki, networkPower, numSectors, sectorSize, sectorInfos) {
			losesAllPrevious := true
			for m := 0; m < n; m++ {
				if winsAtEpoch(t, uint64(m), curr, ki, networkPower, numSectors, sectorSize, sectorInfos) {
					losesAllPrevious = false
					break
				}
			}
			if losesAllPrevious {

				return curr
			}
		}

		// make a new ticket off the previous to keep searching
		curr, err = tm.NextTicket(curr, wAddr, signer)
		require.NoError(t, err)
	}
}

func winsAtEpoch(t *testing.T, epoch uint64, ticket block.Ticket, ki *types.KeyInfo, networkPower, numSectors, sectorSize uint64, sectorInfos ffi.SortedPublicSectorInfo) bool {
	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)
	em := ElectionMachine{}

	// use ticket to seed postRandomness
	postRandomness, err := em.GeneratePoStRandomness(ticket, wAddr, signer, epoch)
	require.NoError(t, err)

	// does this postRandomness create a winner?
	candidates, err := em.GenerateCandidates(postRandomness, sectorInfos, &proofs.ElectionPoster{})
	require.NoError(t, err)

	for _, candidate := range candidates {
		hasher := hasher.NewHasher()
		hasher.Bytes(candidate.PartialTicket[:])
		ct := hasher.Hash()
		if em.CandidateWins(ct, numSectors, 0, networkPower, sectorSize) {
			return true
		}
	}
	return false
}

func losesAtEpoch(t *testing.T, epoch uint64, ticket block.Ticket, ki *types.KeyInfo, networkPower, numSectors, sectorSize uint64, sectorInfos ffi.SortedPublicSectorInfo) bool {
	return !winsAtEpoch(t, epoch, ticket, ki, networkPower, numSectors, sectorSize, sectorInfos)
}

// SeedLoserInNRounds returns a ticket that loses with a null block count of N.
func SeedLoserInNRounds(t *testing.T, n int, ki *types.KeyInfo, networkPower, numSectors, sectorSize uint64) block.Ticket {
	signer := types.NewMockSigner([]types.KeyInfo{*ki})
	wAddr, err := ki.Address()
	require.NoError(t, err)
	tm := TicketMachine{}

	sectorInfos := NFakeSectorInfos(numSectors)
	curr := MakeFakeTicketForTest()

	for {
		if losesAtEpoch(t, uint64(n), curr, ki, networkPower, numSectors, sectorSize, sectorInfos) {
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
	fn  func(block.Ticket)
	fem *FakeElectionMachine
}

var _ ElectionValidator = new(MockElectionMachine)

// NewMockElectionMachine creates a mock given a callback
func NewMockElectionMachine(f func(block.Ticket)) *MockElectionMachine {
	return &MockElectionMachine{fn: f}
}

// GeneratePoStRandomness defers to a fake election machine
func (mem *MockElectionMachine) GeneratePoStRandomness(ticket block.Ticket, candidateAddr address.Address, signer types.Signer, nullBlockCount uint64) ([]byte, error) {
	return mem.fem.GeneratePoStRandomness(ticket, candidateAddr, signer, nullBlockCount)
}

// GenerateCandidates defers to a fake election machine
func (mem *MockElectionMachine) GenerateCandidates(poStRand []byte, sectorInfos ffi.SortedPublicSectorInfo, ep postgenerator.PoStGenerator) ([]ffi.Candidate, error) {
	return mem.fem.GenerateCandidates(poStRand, sectorInfos, ep)
}

// GeneratePoSt defers to a fake election machine
func (mem *MockElectionMachine) GeneratePoSt(sectorInfo ffi.SortedPublicSectorInfo, challengeSeed []byte, winners []ffi.Candidate, ep postgenerator.PoStGenerator) ([]byte, error) {
	return mem.fem.GeneratePoSt(sectorInfo, challengeSeed, winners, ep)
}

// VerifyPoSt defers to fake
func (mem *MockElectionMachine) VerifyPoSt(ep verification.PoStVerifier, allSectorInfos ffi.SortedPublicSectorInfo, sectorSize uint64, challengeSeed []byte, proof []byte, candidates []block.EPoStCandidate, proverID address.Address) (bool, error) {
	return mem.fem.VerifyPoSt(ep, allSectorInfos, sectorSize, challengeSeed, proof, candidates, proverID)
}

// CandidateWins defers to fake
func (mem *MockElectionMachine) CandidateWins(challengeTicket []byte, sectorNum uint64, faultNum uint64, networkPower uint64, sectorSize uint64) bool {
	return mem.fem.CandidateWins(challengeTicket, sectorNum, faultNum, networkPower, sectorSize)
}

// VerifyPoStRandomness runs the callback on the ticket before calling the fake
func (mem *MockElectionMachine) VerifyPoStRandomness(rand block.VRFPi, ticket block.Ticket, candidateAddr address.Address, nullBlockCount uint64) bool {
	mem.fn(ticket)
	return mem.fem.VerifyPoStRandomness(rand, ticket, candidateAddr, nullBlockCount)
}
