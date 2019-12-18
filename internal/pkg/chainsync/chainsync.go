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
	syncer       *syncer.Syncer
	dispatcher   *dispatcher.Dispatcher
	transitionCh chan bool 
}

// NewManager creates a new chain sync manager.
func NewManager(fv syncer.FullBlockValidator, hv syncer.HeaderValidator, cs syncer.ChainSelector, s syncer.ChainReaderWriter, m *chain.MessageStore, f syncer.Fetcher, c clock.Clock) (Manager, error) {
	syncer, err := syncer.NewSyncer(fv, hv, cs, s, m, f, status.NewReporter(), c)
	if err != nil {
		return Manager{}, err
	}
	gapTransitioner := dispatcher.NewGapTransitioner(s, syncer)
	dispatcher := dispatcher.NewDispatcher(syncer, gapTransitioner)
	return Manager{
		syncer:       syncer,
		dispatcher:   dispatcher,
		transitionCh: gapTransitioner.TransitionChannel(),
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

// TransitionChannel returns a channel emitting transition flags.
func (m *Manager) TransitionChannel() chan bool {
	return m.transitionCh
}

// Status returns the block proposer.
func (m *Manager) Status() status.Status {
	return m.syncer.Status()
}
