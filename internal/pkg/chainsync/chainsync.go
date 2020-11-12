package chainsync

import (
	"context"

	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/chain"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/exchange"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/internal/dispatcher"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/internal/syncer"
	"github.com/filecoin-project/venus/internal/pkg/chainsync/status"
	"github.com/filecoin-project/venus/internal/pkg/clock"
	"github.com/filecoin-project/venus/internal/pkg/fork"
	"github.com/filecoin-project/venus/internal/pkg/slashing"
)

// BlockProposer allows callers to propose new blocks for inclusion in the chain.
type BlockProposer interface {
	SendHello(ci *block.ChainInfo) error
	SendOwnBlock(ci *block.ChainInfo) error
	SendGossipBlock(ci *block.ChainInfo) error
	WaiterForTarget(wk block.TipSetKey) func() error
}

// Manager sync the chain.
type Manager struct {
	syncer     *syncer.Syncer
	dispatcher *dispatcher.Dispatcher
}

// NewManager creates a new chain sync manager.
func NewManager(fv syncer.FullBlockValidator,
	hv syncer.BlockValidator,
	cs syncer.ChainSelector,
	s syncer.ChainReaderWriter,
	m *chain.MessageStore,
	bsstore blockstore.Blockstore,
	f syncer.Fetcher,
	exchangeClient exchange.Client,
	c clock.Clock,
	checkPoint block.TipSetKey,
	detector *slashing.ConsensusFaultDetector,
	fork fork.IFork) (Manager, error) {
	syncer, err := syncer.NewSyncer(fv, hv, cs, s, m, bsstore, f, exchangeClient, status.NewReporter(), c, detector, checkPoint, fork)
	if err != nil {
		return Manager{}, err
	}
	gapTransitioner := dispatcher.NewGapTransitioner(s, syncer)
	dispatcher := dispatcher.NewDispatcher(syncer, gapTransitioner)
	return Manager{
		syncer:     syncer,
		dispatcher: dispatcher,
	}, nil
}

// Start starts the chain sync manager.
func (m *Manager) Start(ctx context.Context) error {
	m.dispatcher.Start(ctx)
	return m.syncer.InitStaged()
}

// BlockProposer returns the block proposer.
func (m *Manager) BlockProposer() BlockProposer {
	return m.dispatcher
}

// Status returns the block proposer.
func (m *Manager) Status() status.Status {
	return m.syncer.Status()
}
