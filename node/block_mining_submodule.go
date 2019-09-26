package node

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/mining"
	"github.com/filecoin-project/go-filecoin/protocol/block"
)

// BlockMiningSubmodule enhances the `Node` with block mining capabilities.
type BlockMiningSubmodule struct {
	BlockMiningAPI *block.MiningAPI

	// Mining stuff.
	AddNewlyMinedBlock newBlockFunc
	// cancelMining cancels the context for block production and sector commitments.
	cancelMining    context.CancelFunc
	MiningWorker    mining.Worker
	MiningScheduler mining.Scheduler
	mining          struct {
		sync.Mutex
		isMining bool
	}
	miningDoneWg *sync.WaitGroup
}
