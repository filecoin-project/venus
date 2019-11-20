package chainsync

import (
	"context"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chain"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/dispatcher"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/internal/syncer"
	"github.com/filecoin-project/go-filecoin/internal/pkg/chainsync/status"
	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
)

// BlockProposer allows callers to propose new blocks for inclusion in the chain.
type BlockProposer interface {
	SendHello(ci *block.ChainInfo) error
	SendOwnBlock(ci *block.ChainInfo) error
	SendGossipBlock(ci *block.ChainInfo) error
}

// Manager sync the chain.
type Manager struct {
	syncer     *syncer.Syncer
	dispatcher *dispatcher.Dispatcher
}

// NewManager creates a new chain sync manager.
func NewManager(fv syncer.FullBlockValidator, hv syncer.HeaderValidator, cs syncer.ChainSelector, s syncer.ChainReaderWriter, m *chain.MessageStore, f syncer.Fetcher, c clock.Clock) (Manager, error) {
	syncer, err := syncer.NewSyncer(fv, hv, cs, s, m, f, status.NewReporter(), c)
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
	return m.syncer.StageHead()
}

// BlockProposer returns the block proposer.
func (m *Manager) BlockProposer() BlockProposer {
	return m.dispatcher
}

// Status returns the block proposer.
func (m *Manager) Status() status.Status {
	return m.syncer.Status()
}
