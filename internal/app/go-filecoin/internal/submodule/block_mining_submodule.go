package submodule

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	mining_protocol "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
)

// BlockMiningSubmodule enhances the `Node` with block mining capabilities.
type BlockMiningSubmodule struct {
	BlockMiningAPI *mining_protocol.API

	// Mining stuff.
	AddNewlyMinedBlock newBlockFunc
	// cancelMining cancels the context for block production and sector commitments.
	CancelMining    context.CancelFunc
	MiningWorker    mining.Worker
	MiningScheduler mining.Scheduler
	Mining          struct {
		sync.Mutex
		IsMining bool
	}
	MiningDoneWg *sync.WaitGroup
}

type newBlockFunc func(context.Context, *block.Block)

// NewBlockMiningSubmodule creates a new block mining submodule.
func NewBlockMiningSubmodule(ctx context.Context) (BlockMiningSubmodule, error) {
	return BlockMiningSubmodule{
		// BlockMiningAPI:     nil,
		// AddNewlyMinedBlock: nil,
		// cancelMining:       nil,
		// MiningWorker:       nil,
		// MiningScheduler:    nil,
		// mining:       nil,
		// miningDoneWg: nil,
		// MessageSub:   nil,
	}, nil
}
