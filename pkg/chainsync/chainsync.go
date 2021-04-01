package chainsync

import (
	"context"

	"github.com/filecoin-project/venus/pkg/chainsync/types"
	"github.com/filecoin-project/venus/pkg/consensus"
	types2 "github.com/filecoin-project/venus/pkg/types"

	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/dispatcher"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/chainsync/syncer"
	"github.com/filecoin-project/venus/pkg/clock"
	"github.com/filecoin-project/venus/pkg/fork"
)

// BlockProposer allows callers to propose new blocks for inclusion in the chain.
type BlockProposer interface {
	SetConcurrent(number int64)
	Concurrent() int64
	SyncTracker() *types.TargetTracker
	SendHello(ci *types2.ChainInfo) error
	SendOwnBlock(ci *types2.ChainInfo) error
	SendGossipBlock(ci *types2.ChainInfo) error
}

// Manager sync the chain.
type Manager struct {
	syncer     *syncer.Syncer
	dispatcher *dispatcher.Dispatcher
}

// NewManager creates a new chain sync manager.
func NewManager(fv syncer.StateProcessor,
	hv *consensus.BlockValidator,
	cs syncer.ChainSelector,
	s syncer.ChainReaderWriter,
	m *chain.MessageStore,
	bsstore blockstore.Blockstore,
	exchangeClient exchange.Client,
	c clock.Clock,
	fork fork.IFork) (Manager, error) {
	syncer, err := syncer.NewSyncer(fv, hv, cs, s, m, bsstore, exchangeClient, c, fork)
	if err != nil {
		return Manager{}, err
	}

	dispatcher := dispatcher.NewDispatcher(syncer)

	return Manager{
		syncer:     syncer,
		dispatcher: dispatcher,
	}, nil
}

// Start starts the chain sync manager.
func (m *Manager) Start(ctx context.Context) error {
	m.dispatcher.Start(ctx)
	return nil
}

// BlockProposer returns the block proposer.
func (m *Manager) BlockProposer() BlockProposer {
	return m.dispatcher
}
