package consensus

import (
	"context"
	"fmt"
	"testing"

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	acrypto "github.com/filecoin-project/specs-actors/actors/crypto"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
	"github.com/filecoin-project/go-filecoin/internal/pkg/crypto"
	"github.com/filecoin-project/go-filecoin/internal/pkg/drand"
	"github.com/filecoin-project/go-filecoin/internal/pkg/postgenerator"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// RequireNewTipSet instantiates and returns a new tipset of the given blocks
// and requires that the setup validation succeed.
func RequireNewTipSet(require *require.Assertions, blks ...*block.Block) block.TipSet {
	ts, err := block.NewTipSet(blks...)
	require.NoError(err)
	return ts
}

// FakeConsensusStateViewer is a fake power state viewer.
type FakeConsensusStateViewer struct {
	Views map[cid.Cid]*state.FakeStateView
}

// PowerStateView returns the state view for a root.
func (f *FakeConsensusStateViewer) PowerStateView(root cid.Cid) PowerStateView {
	return f.Views[root]
}

// FaultStateView returns the state view for a root.
func (f *FakeConsensusStateViewer) FaultStateView(root cid.Cid) FaultStateView {
	return f.Views[root]
}

// FakeMessageValidator is a validator that doesn't validate to simplify message creation in tests.
type FakeMessageValidator struct{}

// Validate always returns nil
func (tsmv *FakeMessageValidator) Validate(ctx context.Context, msg *types.UnsignedMessage, fromActor *actor.Actor) error {
	return nil
}

// FakeElectionMachine generates fake election proofs and verifies all proofs
type FakeElectionMachine struct{}

var _ ElectionValidator = new(FakeElectionMachine)

// GenerateElectionProof returns a fake randomness
func (fem *FakeElectionMachine) GenerateElectionProof(_ context.Context, _ *drand.Entry,
	_ abi.ChainEpoch, _ address.Address, _ address.Address, _ types.Signer) (crypto.VRFPi, error) {
	return MakeFakeVRFProofForTest(), nil
}

// GenerateEPoSt returns a fake post proof
func (fem *FakeElectionMachine) GenerateWinningPoSt(ctx context.Context, allSectorInfos []abi.SectorInfo, entry *drand.Entry, epoch abi.ChainEpoch, ep postgenerator.PoStGenerator, maddr address.Address) ([]block.PoStProof, error) {
	return []block.PoStProof{{
		RegisteredProof: constants.DevRegisteredWinningPoStProof,
		ProofBytes:      []byte{0xe},
	}}, nil
}

func (fem *FakeElectionMachine) IsWinner(challengeTicket []byte, minerPower, networkPower abi.StoragePower) bool {
	return true
}

func (fem *FakeElectionMachine) VerifyElectionProof(_ context.Context, _ *drand.Entry, _ abi.ChainEpoch, _ address.Address, _ address.Address, _ crypto.VRFPi) error {
	return nil
}

func (fem *FakeElectionMachine) VerifyWinningPoSt(_ context.Context, _ EPoStVerifier, _ []abi.SectorInfo, _ *drand.Entry, _ abi.ChainEpoch, _ []block.PoStProof, _ address.Address) (bool, error) {
	return true, nil
}

// FakeTicketMachine generates fake tickets and verifies all tickets
type FakeTicketMachine struct{}

// MakeTicket returns a fake ticket
func (ftm *FakeTicketMachine) MakeTicket(ctx context.Context, base block.TipSetKey, epoch abi.ChainEpoch, miner address.Address, entry *drand.Entry, newPeriod bool, worker address.Address, signer types.Signer) (block.Ticket, error) {
	return MakeFakeTicketForTest(), nil
}

// IsValidTicket always returns true
func (ftm *FakeTicketMachine) IsValidTicket(ctx context.Context, base block.TipSetKey, entry *drand.Entry, newPeriod bool,
	epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error {
	return nil
}

// FailingTicketValidator marks all tickets as invalid
type FailingTicketValidator struct{}

// IsValidTicket always returns false
func (ftv *FailingTicketValidator) IsValidTicket(ctx context.Context, base block.TipSetKey, entry *drand.Entry, newPeriod bool,
	epoch abi.ChainEpoch, miner address.Address, workerSigner address.Address, ticket block.Ticket) error {
	return fmt.Errorf("invalid ticket")
}

// FailingElectionValidator marks all election candidates as invalid
type FailingElectionValidator struct{}

var _ ElectionValidator = new(FailingElectionValidator)

func (fev *FailingElectionValidator) IsWinner(challengeTicket []byte, minerPower, networkPower abi.StoragePower) bool {
	return false
}

func (fev *FailingElectionValidator) VerifyElectionProof(_ context.Context, _ *drand.Entry, _ abi.ChainEpoch, _ address.Address, _ address.Address, _ crypto.VRFPi) error {
	return nil
}

func (fev *FailingElectionValidator) VerifyWinningPoSt(_ context.Context, _ EPoStVerifier, _ []abi.SectorInfo, _ *drand.Entry, _ abi.ChainEpoch, _ []block.PoStProof, _ address.Address) (bool, error) {
	return true, nil
}

// MakeFakeTicketForTest creates a fake ticket
func MakeFakeTicketForTest() block.Ticket {
	val := make([]byte, 65)
	val[0] = 200
	return block.Ticket{
		VRFProof: crypto.VRFPi(val[:]),
	}
}

// MakeFakeVRFProofForTest creates a fake election proof
func MakeFakeVRFProofForTest() []byte {
	proof := make([]byte, 65)
	proof[0] = 42
	return proof
}

// MakeFakePoStForTest creates a fake post
func MakeFakePoStsForTest() []block.PoStProof {
	return []block.PoStProof{{
		RegisteredProof: constants.DevRegisteredWinningPoStProof,
		ProofBytes:      []byte{0xe},
	}}
}

// NFakeSectorInfos returns numSectors fake sector infos
func RequireFakeSectorInfos(t *testing.T, numSectors uint64) []abi.SectorInfo {
	var infos []abi.SectorInfo
	for i := uint64(0); i < numSectors; i++ {
		infos = append(infos, abi.SectorInfo{
			RegisteredProof: constants.DevRegisteredSealProof,
			SectorNumber:    abi.SectorNumber(i),
			SealedCID:       types.CidFromString(t, fmt.Sprintf("fake-sector-%d", i)),
		})
	}

	return infos
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

type FakeSampler struct {
	Seed uint
}

func (s *FakeSampler) SampleTicket(_ context.Context, _ block.TipSetKey, epoch abi.ChainEpoch) (block.Ticket, error) {
	return block.Ticket{
		VRFProof: []byte(fmt.Sprintf("s=%d,e=%d", s.Seed, epoch)),
	}, nil
}
