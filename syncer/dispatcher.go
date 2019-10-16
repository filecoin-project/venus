package syncer

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/consensus"
	"github.com/filecoin-project/go-filecoin/metrics"
	"github.com/filecoin-project/go-filecoin/metrics/tracing"
	"github.com/filecoin-project/go-filecoin/net"
	"github.com/filecoin-project/go-filecoin/types"
)

// Possible FSM state values
type FSMState int

const (
	INIT = iota
	BOOTSTRAP
	SNAPSHOT
	CATCHUP
	FOLLOW
)


// Dispatcher manages dispatch of syncing validation/propagation pipelines.
// It manages the filecoin chain syncing FSM.  It consults the node's target
// heads, partition detection, and chain finality information to dispatch
// syncing requests and transition the FSM.
type Dispatcher struct {
	state FSMState
	
	// informants consulted by the dispatcher to issue new pipeline
	// executions and transition the FSM.
	chainReader dispatcherChainReader
	targeter    Targeter
	clock       clock.Clock
	partitions  PartitionDetector

	// syncing pipelines run against new syncing targets
	init        Pipeline
	bootstrap   Pipeline
	snapshot    Pipeline
	catchup     Pipeline
	follow      Pipeline
}

// Pipeline is the interface syncing validation/propagation pipelines fullfill
// to be called by the dispatcher.  Pipelines are inherently side-effecting,
// they propagate messages through the network and new blocks through the
// local chain system based on the success of a series of validation stages.
type Pipeline interface {
	HandleNewRequest(req *PipelineRequest) error
}

// NewDispatcher constructs a dispatcher that tracks syncing targets and runs
// syncing pipelines in response
func NewDispatcher(t Targeter, pd PartitionDetector, ch dispatcherChainReader, c clock.Clock, ipl, bpl, spl, cpl, fpl Pipeline) *Syncer {
	return &Dispatcher{
		state:       INIT,
		chainReader: ch,
		targeter:    t,
		clock:       c,
		partitions:  pd, 
		init:        ipl,
		bootstrap:   bpl,
		snapshot:    spl,
		catchup:     cpl,
		follow:      fpl
		
	}
}

// Run kicks off chain syncing.  The dispatcher begins in the init state.
func (d *Dispatcher) Run() {
	go func() {
		for {
			nextSyncRequest := d.getNextTarget()
			nextSyncResult, err := d.dispatch(nextSyncRequest)
			log.Errorf("unexpected error while syncing %s", nextSyncRequest.String())
			d.updateState(nextSyncResult)
		}
		
	}()
}

// updateState incorporates results of syncing into the FSM state and the
// targeter's bad block cache 
func (d *Dispatcher) updateState(res *PipelinResult)


// PubsubValidationFunction delegates pubsub validation to the current pipeline
// in order to perform this validation stage exactly once
func (d *Dispatcher) PubsubValidationFunction() {
	
}

