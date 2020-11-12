package testhelpers

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	acrypto "github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/consensus"
	"github.com/filecoin-project/venus/internal/pkg/state"
	"github.com/filecoin-project/venus/internal/pkg/types"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// FakeWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type FakeWorkerPorcelainAPI struct {
	blockTime time.Duration
	stateView *state.FakeStateView
	rnd       consensus.ChainRandomness
}

// NewDefaultFakeWorkerPorcelainAPI returns a FakeWorkerPorcelainAPI.
func NewDefaultFakeWorkerPorcelainAPI(signer address.Address, rnd consensus.ChainRandomness) *FakeWorkerPorcelainAPI {
	return &FakeWorkerPorcelainAPI{
		blockTime: BlockTimeTest,
		stateView: &state.FakeStateView{
			Power: &state.NetworkPower{
				RawBytePower:         big.NewInt(1),
				QualityAdjustedPower: big.NewInt(1),
				MinerCount:           0,
				MinPowerMinerCount:   0,
			},
			Miners: map[address.Address]*state.FakeMinerState{},
		},
		rnd: rnd,
	}
}

// NewFakeWorkerPorcelainAPI produces an api suitable to use as the worker's porcelain api.
func NewFakeWorkerPorcelainAPI(rnd consensus.ChainRandomness, totalPower uint64, minerToWorker map[address.Address]address.Address) *FakeWorkerPorcelainAPI {
	f := &FakeWorkerPorcelainAPI{
		blockTime: BlockTimeTest,
		stateView: &state.FakeStateView{
			Power: &state.NetworkPower{
				RawBytePower:         big.NewIntUnsigned(totalPower),
				QualityAdjustedPower: big.NewIntUnsigned(totalPower),
				MinerCount:           0,
				MinPowerMinerCount:   0,
			},
			Miners: map[address.Address]*state.FakeMinerState{},
		},
		rnd: rnd,
	}
	for k, v := range minerToWorker {
		f.stateView.Miners[k] = &state.FakeMinerState{
			Owner:             v,
			Worker:            v,
			ClaimedRawPower:   big.Zero(),
			ClaimedQAPower:    big.Zero(),
			PledgeRequirement: big.Zero(),
			PledgeBalance:     big.Zero(),
		}
	}
	return f
}

// BlockTime returns the blocktime FakeWorkerPorcelainAPI is configured with.
func (t *FakeWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
}

// PowerStateView returns the state view.
func (t *FakeWorkerPorcelainAPI) PowerStateView(_ block.TipSetKey) (state.PowerStateView, error) {
	return t.stateView, nil
}

func (t *FakeWorkerPorcelainAPI) FaultsStateView(_ block.TipSetKey) (state.FaultStateView, error) {
	return t.stateView, nil
}

func (t *FakeWorkerPorcelainAPI) SampleChainRandomness(ctx context.Context, head block.TipSetKey, tag acrypto.DomainSeparationTag,
	epoch abi.ChainEpoch, entropy []byte) (abi.Randomness, error) {
	return t.rnd.SampleChainRandomness(ctx, head, tag, epoch, entropy)
}

// MakeCommitment creates a random commitment.
func MakeCommitment() []byte {
	return MakeRandomBytes(32)
}

// MakeCommitments creates three random commitments for constructing a
// types.Commitments.
func MakeCommitments() types.Commitments {
	comms := types.Commitments{}
	copy(comms.CommD[:], MakeCommitment()[:])
	copy(comms.CommR[:], MakeCommitment()[:])
	copy(comms.CommRStar[:], MakeCommitment()[:])
	return comms
}

// MakeRandomBytes generates a randomized byte slice of size 'size'
func MakeRandomBytes(size int) []byte {
	comm := make([]byte, size)
	if _, err := rand.Read(comm); err != nil {
		panic(err)
	}

	return comm
}
