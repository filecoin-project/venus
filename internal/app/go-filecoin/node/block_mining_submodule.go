package node

import (
	"context"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/mining"
	mining_protocol "github.com/filecoin-project/go-filecoin/internal/pkg/protocol/mining"
)

// BlockMiningSubmodule enhances the `Node` with block mining capabilities.
type BlockMiningSubmodule struct {
	BlockMiningAPI *mining_protocol.API

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
