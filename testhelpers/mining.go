package testhelpers

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// TestWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type TestWorkerPorcelainAPI struct {
	blockTime     time.Duration
	workerAddr    address.Address
	totalPower    uint64
	minerToWorker map[address.Address]address.Address
}

// NewDefaultTestWorkerPorcelainAPI returns a TestWorkerPorcelainAPI.
func NewDefaultTestWorkerPorcelainAPI(signer address.Address) *TestWorkerPorcelainAPI {
	return &TestWorkerPorcelainAPI{
		blockTime:  BlockTimeTest,
		workerAddr: signer,
		totalPower: 1,
	}
}

// NewTestWorkerPorcelainAPI produces an api suitable to use as the worker's porcelain api.
func NewTestWorkerPorcelainAPI(signer address.Address, totalPower uint64, minerToWorker map[address.Address]address.Address) *TestWorkerPorcelainAPI {
	return &TestWorkerPorcelainAPI{
		blockTime:     BlockTimeTest,
		workerAddr:    signer,
		totalPower:    totalPower,
		minerToWorker: minerToWorker,
	}
}

// BlockTime returns the blocktime TestWorkerPorcelainAPI is configured with.
func (t *TestWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
}

// MinerGetWorkerAddress returns the worker address set in TestWorkerPorcelainAPI
func (t *TestWorkerPorcelainAPI) MinerGetWorkerAddress(_ context.Context, _ address.Address, _ types.TipSetKey) (address.Address, error) {
	return t.workerAddr, nil
}

// Queryer returns a queryer object for the given tipset
func (t *TestWorkerPorcelainAPI) Queryer(ctx context.Context, tsk types.TipSetKey) (consensus.ActorStateQueryer, error) {
	return &TestQueryer{
		totalPower:    t.totalPower,
		minerToWorker: t.minerToWorker,
	}, nil
}

// TestQueryer is used when testing with a PowerTableView
type TestQueryer struct {
	totalPower    uint64
	minerToWorker map[address.Address]address.Address
}

// Query produces preconfigured results for PowerTableView queries
func (tq *TestQueryer) Query(ctx context.Context, optFrom, to address.Address, method string, params ...interface{}) ([][]byte, error) {
	if method == "getTotalStorage" {
		return [][]byte{types.NewBytesAmount(tq.totalPower).Bytes()}, nil
	} else if method == "getPower" {
		// always return 1
		return [][]byte{types.NewBytesAmount(1).Bytes()}, nil
	} else if method == "getWorker" {
		if tq.minerToWorker != nil {
			return [][]byte{tq.minerToWorker[to].Bytes()}, nil
		}
		// just return the miner address
		return [][]byte{to.Bytes()}, nil
	}
	return [][]byte{}, fmt.Errorf("unknown method for TestQueryer '%s'", method)
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
