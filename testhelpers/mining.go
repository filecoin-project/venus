package testhelpers

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

// BlockTimeTest is the block time used by workers during testing.
const BlockTimeTest = time.Second

// TestWorkerPorcelainAPI implements the WorkerPorcelainAPI>
type TestWorkerPorcelainAPI struct {
	blockTime  time.Duration
	workerAddr address.Address
}

// NewDefaultTestWorkerPorcelainAPI returns a TestWorkerPorcelainAPI.
func NewDefaultTestWorkerPorcelainAPI(signer address.Address) *TestWorkerPorcelainAPI {
	return &TestWorkerPorcelainAPI{blockTime: BlockTimeTest, workerAddr: signer}
}

// BlockTime returns the blocktime TestWorkerPorcelainAPI is configured with.
func (t *TestWorkerPorcelainAPI) BlockTime() time.Duration {
	return t.blockTime
}

// MinerGetWorkerAddress returns the worker address set in TestWorkerPorcelainAPI
func (t *TestWorkerPorcelainAPI) MinerGetWorkerAddress(ctx context.Context, minerAddr address.Address) (address.Address, error) {
	return t.workerAddr, nil
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
