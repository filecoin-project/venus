package testhelpers

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/consensus"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// FakeWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type FakeWorkerPorcelainAPI struct {
	blockTime     time.Duration
	workerAddr    address.Address
	totalPower    uint64
	minerToWorker map[address.Address]address.Address
}

// NewDefaultFakeWorkerPorcelainAPI returns a FakeWorkerPorcelainAPI.
func NewDefaultFakeWorkerPorcelainAPI(signer address.Address) *FakeWorkerPorcelainAPI {
	return &FakeWorkerPorcelainAPI{
		blockTime:  BlockTimeTest,
		workerAddr: signer,
		totalPower: 1,
	}
}

// NewFakeWorkerPorcelainAPI produces an api suitable to use as the worker's porcelain api.
func NewFakeWorkerPorcelainAPI(signer address.Address, totalPower uint64, minerToWorker map[address.Address]address.Address) *FakeWorkerPorcelainAPI {
	return &FakeWorkerPorcelainAPI{
		blockTime:     BlockTimeTest,
		workerAddr:    signer,
		totalPower:    totalPower,
		minerToWorker: minerToWorker,
	}
}

// BlockTime returns the blocktime FakeWorkerPorcelainAPI is configured with.
func (t *FakeWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
}

// MinerGetWorkerAddress returns the worker address set in FakeWorkerPorcelainAPI
func (t *FakeWorkerPorcelainAPI) MinerGetWorkerAddress(_ context.Context, _ address.Address, _ block.TipSetKey) (address.Address, error) {
	return t.workerAddr, nil
}

// Snapshot returns a snapshot object for the given tipset
func (t *FakeWorkerPorcelainAPI) Snapshot(ctx context.Context, tsk block.TipSetKey) (consensus.ActorStateSnapshot, error) {
	return &consensus.FakePowerTableViewSnapshot{
		MinerPower:    types.NewBytesAmount(1),
		TotalPower:    types.NewBytesAmount(t.totalPower),
		MinerToWorker: t.minerToWorker,
	}, nil
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
