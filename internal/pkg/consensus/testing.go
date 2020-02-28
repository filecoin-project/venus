package consensus

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/minio/blake2b-simd"
	"github.com/stretchr/testify/require"

	ffi "github.com/filecoin-project/filecoin-ffi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
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

// GenerateEPoStVrfProof returns a fake post randomness byte array
func (fem *FakeElectionMachine) GenerateEPoStVrfProof(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (block.VRFPi, error) {
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

// GenerateEPoSt returns a fake post proof
func (fem *FakeElectionMachine) GenerateEPoSt(_ ffi.SortedPublicSectorInfo, _ []byte, _ []ffi.Candidate, _ postgenerator.PoStGenerator) ([]byte, error) {
	return MakeFakePoStForTest(), nil
}

// VerifyEPoStVrfProof returns true
func (fem *FakeElectionMachine) VerifyEPoStVrfProof(context.Context, block.TipSetKey, abi.ChainEpoch, address.Address, address.Address, block.VRFPi) error {
	return nil
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

// MakeTicket returns a fake ticket
func (ftm *FakeTicketMachine) MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, signer types.Signer) (block.Ticket, error) {
	return MakeFakeTicketForTest(), nil
}

// IsValidTicket always returns true
func (ftm *FakeTicketMachine) IsValidTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, ticket block.Ticket) error {
	return nil
}

// FailingTicketValidator marks all tickets as invalid
type FailingTicketValidator struct{}

// IsValidTicket always returns false
func (ftv *FailingTicketValidator) IsValidTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, worker address.Address, ticket block.Ticket) error {
	return fmt.Errorf("invalid ticket")
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

// VerifyEPoStVrfProof return true
func (fev *FailingElectionValidator) VerifyEPoStVrfProof(context.Context, block.TipSetKey, abi.ChainEpoch, address.Address, address.Address, block.VRFPi) error {
	return nil
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

// SeedFirstWinnerInNRounds returns seeded fake chain randomness that when mined upon for N rounds
// by a miner that has `minerPower` out of a system-wide `totalPower` and keyinfo
// `ki` will produce a ticket that gives a winning election proof in exactly `n`
// rounds.  No wins before n rounds are up.
//
// Note that this is a deterministic function of the inputs as we return the
// first such seed.
//
// Note that there are no guarantees that this function will terminate on new
// inputs as miner power might be so low that winning a ticket is very
// unlikely.  However runtime is deterministic so if it runs fast once on
// given inputs is safe to use in tests.
func SeedFirstWinnerInNRounds(t *testing.T, n int, miner address.Address, ki *crypto.KeyInfo, networkPower, numSectors, sectorSize uint64) *FakeChainRandomness {
	signer := types.NewMockSigner([]crypto.KeyInfo{*ki})
	worker, err := ki.Address()
	require.NoError(t, err)

	// give it some fake sector infos
	sectorInfos := NFakeSectorInfos(numSectors)
	head := block.NewTipSetKey() // The fake chain randomness doesn't actually inspect any tipsets

	rnd := &FakeChainRandomness{Seed: 1}
	em := NewElectionMachine(rnd)

	for {
		if winsAtEpoch(t, em, head, abi.ChainEpoch(n), miner, worker, signer, networkPower, numSectors, sectorSize, sectorInfos) {
			losesAllPrevious := true
			for m := 0; m < n; m++ {
				if winsAtEpoch(t, em, head, abi.ChainEpoch(m), miner, worker, signer, networkPower, numSectors, sectorSize, sectorInfos) {
					losesAllPrevious = false
					break
				}
			}
			if losesAllPrevious {
				return rnd
			}
		}
		// Try a different seed.
		rnd.Seed++
	}
}

func winsAtEpoch(t *testing.T, em *ElectionMachine, head block.TipSetKey, epoch abi.ChainEpoch, miner, worker address.Address,
	signer types.Signer, networkPower, numSectors, sectorSize uint64, sectorInfos ffi.SortedPublicSectorInfo) bool {

	epostVRFProof, err := em.GenerateEPoStVrfProof(context.Background(), head, epoch, miner, worker, signer)
	require.NoError(t, err)
	digest := blake2b.Sum256(epostVRFProof)

	// does this postRandomness create a winner?
	candidates, err := em.GenerateCandidates(digest[:], sectorInfos, &proofs.ElectionPoster{})
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

///// Sampler /////

// FakeChainRandomness generates deterministic values that are a function of a seed and the provided
// tag, epoch, and entropy (but *not* the chain head key).
type FakeChainRandomness struct {
	Seed uint
}

func (s *FakeChainRandomness) SampleChainRandomness(_ context.Context, _ block.TipSetKey, tag acrypto.DomainSeparationTag, epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return []byte(fmt.Sprintf("s=%d,e=%d,t=%d,p=%s", s.Seed, epoch, tag, string(entropy))), nil
}
