package chainsync

import (
	"context"

	chain2 "github.com/filecoin-project/venus/app/submodule/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/pkg/chainsync/types"
	"github.com/filecoin-project/venus/pkg/consensus"
	"github.com/filecoin-project/venus/pkg/statemanger"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	types2 "github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/pkg/chainsync/dispatcher"
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

var _ = (BlockProposer)((*dispatcher.Dispatcher)(nil))

// Manager sync the chain.
type Manager struct {
	dispatcher *dispatcher.Dispatcher
}

// NewManager creates a new chain sync manager.
func NewManager(
	stmgr *statemanger.Stmgr,
	hv *consensus.BlockValidator,
	submodule *chain2.ChainSubmodule,
	cs syncer.ChainSelector,
	bsstore blockstoreutil.Blockstore,
	exchangeClient exchange.Client,
	c clock.Clock,
	fork fork.IFork) (Manager, error) {

	chainSyncer, err := syncer.NewSyncer(stmgr, hv, cs, submodule.ChainReader,
		submodule.MessageStore, bsstore,
		exchangeClient, c, fork)
	if err != nil {
		return Manager{}, err
	}

	return Manager{
		dispatcher: dispatcher.NewDispatcher(chainSyncer),
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
