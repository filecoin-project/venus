package consensus

import (
	"bytes"
	"context"
	"fmt"
	"github.com/filecoin-project/venus/internal/pkg/constants"

	proof2 "github.com/filecoin-project/specs-actors/v2/actors/runtime/proof" // todo ref lotus

	address "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/crypto"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/consensus/lib/sigs"
	crypto2 "github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/internal/pkg/specactors/builtin/miner"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/util/ffiwrapper"
)

// Interface to PoSt verification, modify by force EPoStVerifier -> ProofVerifier
type ProofVerifier interface {
	VerifySeal(info proof2.SealVerifyInfo) (bool, error)
	VerifyWinningPoSt(ctx context.Context, info proof2.WinningPoStVerifyInfo) (bool, error)
	VerifyWindowPoSt(ctx context.Context, info proof2.WindowPoStVerifyInfo) (bool, error)
	GenerateWinningPoStSectorChallenge(ctx context.Context, proofType abi.RegisteredPoStProof, minerID abi.ActorID, randomness abi.PoStRandomness, eligibleSectorCount uint64) ([]uint64, error)
}

type SectorsStateView interface {
	MinerSectorConfiguration(ctx context.Context, maddr address.Address) (*state.MinerSectorConfiguration, error)
	MinerGetSector(ctx context.Context, maddr address.Address, sectorNum abi.SectorNumber) (*miner.SectorOnChainInfo, bool, error)
}

type MiningCheckAPI interface {
	ChainGetRandomnessFromBeacon(ctx context.Context, tsk block.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)
	ChainGetRandomnessFromTickets(ctx context.Context, tsk block.TipSetKey, personalization crypto.DomainSeparationTag, randEpoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error)

	MinerGetBaseInfo(context.Context, address.Address, abi.ChainEpoch, block.TipSetKey) (block.MiningBaseInfo, error)

	WalletSign(context.Context, address.Address, []byte) (*crypto.Signature, error)
}

func DefaultProofVerifier() ffiwrapper.Verifier {
	return ffiwrapper.ProofVerifier
}

func IsRoundWinner(ctx context.Context, ts *block.TipSet, round abi.ChainEpoch,
	miner address.Address, brand block.BeaconEntry, mbi *block.MiningBaseInfo, a MiningCheckAPI) (*crypto2.ElectionProof, error) {

	buf := new(bytes.Buffer)
	if err := miner.MarshalCBOR(buf); err != nil {
		return nil, xerrors.Errorf("failed to cbor marshal address: %w", err)
	}

	electionRand, err := chain.DrawRandomness(brand.Data, crypto.DomainSeparationTag_ElectionProofProduction, round, buf.Bytes())
	if err != nil {
		return nil, xerrors.Errorf("failed to draw randomness: %w", err)
	}

	vrfout, err := ComputeVRF(ctx, a.WalletSign, mbi.WorkerKey, electionRand)
	if err != nil {
		return nil, xerrors.Errorf("failed to compute VRF: %w", err)
	}

	ep := &crypto2.ElectionProof{VRFProof: vrfout}
	j := ep.ComputeWinCount(mbi.MinerPower, mbi.NetworkPower)
	ep.WinCount = j
	if j < 1 {
		return nil, nil
	}

	return ep, nil
}

type SignFunc func(context.Context, address.Address, []byte) (*crypto.Signature, error)

func VerifyVRF(ctx context.Context, worker address.Address, vrfBase, vrfproof []byte) error {
	_, span := trace.StartSpan(ctx, "VerifyVRF")
	defer span.End()

	sig := &crypto.Signature{
		Type: crypto.SigTypeBLS,
		Data: vrfproof,
	}

	if err := sigs.Verify(sig, worker, vrfBase); err != nil {
		return xerrors.Errorf("vrf was invalid: %w", err)
	}

	return nil
}

func ComputeVRF(ctx context.Context, sign SignFunc, worker address.Address, sigInput []byte) ([]byte, error) {
	sig, err := sign(ctx, worker, sigInput)
	if err != nil {
		return nil, err
	}

	if sig.Type != crypto.SigTypeBLS {
		return nil, fmt.Errorf("miner worker address was not a BLS key")
	}

	return sig.Data, nil
}

func VerifyElectionPoStVRF(ctx context.Context, worker address.Address, rand []byte, evrf []byte) error {
	if constants.InsecurePoStValidation {
		return nil
	}
	return VerifyVRF(ctx, worker, rand, evrf)
}
