package testhelpers

import (
	"crypto/rand"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/state"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// FakeWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type FakeWorkerPorcelainAPI struct {
	blockTime time.Duration
	stateView *state.FakeStateView
}

// NewDefaultFakeWorkerPorcelainAPI returns a FakeWorkerPorcelainAPI.
func NewDefaultFakeWorkerPorcelainAPI(signer address.Address) *FakeWorkerPorcelainAPI {
	return &FakeWorkerPorcelainAPI{
		blockTime: BlockTimeTest,
		stateView: &state.FakeStateView{
			NetworkPower: abi.NewStoragePower(1),
			Miners:       map[address.Address]*state.FakeMinerState{},
		},
	}
}

// NewFakeWorkerPorcelainAPI produces an api suitable to use as the worker's porcelain api.
func NewFakeWorkerPorcelainAPI(signer address.Address, totalPower uint64, minerToWorker map[address.Address]address.Address) *FakeWorkerPorcelainAPI {
	f := &FakeWorkerPorcelainAPI{
		blockTime: BlockTimeTest,
		stateView: &state.FakeStateView{
			NetworkPower: abi.NewStoragePower(int64(totalPower)),
			Miners:       map[address.Address]*state.FakeMinerState{},
		},
	}
	for k, v := range minerToWorker {
		f.stateView.Miners[k] = &state.FakeMinerState{
			Owner:  v,
			Worker: v,
		}
	}
	return f
}

// BlockTime returns the blocktime FakeWorkerPorcelainAPI is configured with.
func (t *FakeWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
}

// PowerStateView returns the state view.
func (t *FakeWorkerPorcelainAPI) PowerStateView(_ block.TipSetKey) (consensus.PowerStateView, error) {
	return t.stateView, nil
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
