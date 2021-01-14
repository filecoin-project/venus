package dispatcher

import (
	"context"
	"github.com/filecoin-project/venus/pkg/chainsync/types"
	"runtime/debug"
	"time"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/pkg/block"
	"github.com/filecoin-project/venus/pkg/util/moresync"
)

var log = logging.Logger("chainsync.dispatcher")

// DefaultInQueueSize is the bucketSize of the channel used for receiving targets from producers.
const DefaultInQueueSize = 5

// DefaultWorkQueueSize is the bucketSize of the work queue
const DefaultWorkQueueSize = 5

// dispatchSyncer is the interface of the logic syncing incoming chains
type dispatchSyncer interface {
	Staged() *block.TipSet
	HandleNewTipSet(context.Context, *types.Target) error
}

// NewDispatcher creates a new syncing dispatcher with default queue sizes.
func NewDispatcher(catchupSyncer dispatchSyncer) *Dispatcher {
	return NewDispatcherWithSizes(catchupSyncer, DefaultWorkQueueSize, DefaultInQueueSize)
}

// NewDispatcherWithSizes creates a new syncing dispatcher.
func NewDispatcherWithSizes(syncer dispatchSyncer, workQueueSize, inQueueSize int) *Dispatcher {
	return &Dispatcher{
		workTracker:  types.NewTargetTracker(workQueueSize),
		syncer:       syncer,
		incoming:     make(chan *types.Target, inQueueSize),
		control:      make(chan interface{}, 1),
		registeredCb: func(t *types.Target, err error) {},
	}
}

// cbMessage registers a user callback to be fired following every successful
// sync.
type cbMessage struct {
	cb func(*types.Target, error)
}

// Dispatcher receives, sorts and dispatches targets to the catchupSyncer to control
// chain syncing.
//
// New targets arrive over the incoming channel. The dispatcher then puts them
// into the workTracker which sorts them by their claimed chain height. The
// dispatcher pops the highest priority target from the queue and then attempts
// to sync the target using its internal catchupSyncer.
//
// The dispatcher has a simple control channel. It reads this for external
// controls. Currently there is only one kind of control message.  It registers
// a callback that the dispatcher will call after every non-erroring sync.
type Dispatcher struct {
	// workTracker is a priority queue of target chain heads that should be
	// synced
	workTracker *types.TargetTracker
	// incoming is the queue of incoming sync targets to the dispatcher.
	incoming chan *types.Target
	// syncer is used for dispatching sync targets for chain heads to sync
	// local chain state to these targets.
	syncer dispatchSyncer

	// registeredCb is a callback registered over the control channel.  It
	// is called after every successful sync.
	registeredCb func(*types.Target, error)
	// control is a queue of control messages not yet processed.
	control chan interface{}

	// syncTargetCount counts the number of successful syncs.
	syncTargetCount uint64
}

// SendOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) SyncTracker() *types.TargetTracker {
	return d.workTracker
}

// SendHello handles chain information from bootstrap peers.
func (d *Dispatcher) SendHello(ci *block.ChainInfo) error {
	return d.addTracker(ci)
}

// SendOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) SendOwnBlock(ci *block.ChainInfo) error {
	return d.addTracker(ci)
}

// SendGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) SendGossipBlock(ci *block.ChainInfo) error {
	return d.addTracker(ci)
}

func (d *Dispatcher) addTracker(ci *block.ChainInfo) error {
	d.incoming <- &types.Target{
		ChainInfo: *ci,
		Base:      d.syncer.Staged(),
		Start:     time.Now(),
	}
	return nil
}

// Start launches the business logic for the syncing subsystem.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	go func() {
		defer func() {
			log.Error("exiting")
			if r := recover(); r != nil {
				log.Errorf("panic: %v", r)
				debug.PrintStack()
			}
		}()

		for {
			// Handle shutdown
			select {
			case <-syncingCtx.Done():
				log.Info("context done")
				return
			case ctrl := <-d.control:
				log.Infof("processing control: %v", ctrl)
				d.processCtrl(ctrl)
			default:
			}

			// Handle incoming targets
			select {
			case target := <-d.incoming:
				// Sort new targets by putting on work queue.
				d.workTracker.Add(target)
				log.Infof("received %s height %d current work len %d  incoming len: %v", target.Head.Key(), target.Head.EnsureHeight(), d.workTracker.Len(), len(d.incoming))
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Millisecond * 50)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				syncTarget, popped := d.workTracker.Select()
				if popped {
					// Do work
					err := d.syncer.HandleNewTipSet(syncingCtx, syncTarget)
					d.workTracker.Remove(syncTarget)
					if err != nil {
						log.Debugf("failed sync of %v at %d  %s", syncTarget.Head.Key(), syncTarget.Head.EnsureHeight(), err)
					}
					d.syncTargetCount++
					d.registeredCb(syncTarget, err)
				}
			}
		}
	}()
}

// RegisterCallback registers a callback on the dispatcher that
// will fire after every successful target sync.
func (d *Dispatcher) RegisterCallback(cb func(*types.Target, error)) {
	d.control <- cbMessage{cb: cb}
}

// WaiterForTarget returns a function that will block until the dispatcher
// processes the given target and returns the error produced by that targer
func (d *Dispatcher) WaiterForTarget(waitKey block.TipSetKey) func() error {
	processed := moresync.NewLatch(1)
	var syncErr error
	d.RegisterCallback(func(t *types.Target, err error) {
		if t.ChainInfo.Head.Key().Equals(waitKey) {
			syncErr = err
			processed.Done()
		}
	})
	return func() error {
		processed.Wait()
		return syncErr
	}
}
func (d *Dispatcher) processCtrl(ctrlMsg interface{}) {
	// processCtrl takes a control message, determines its type, and performs the
	// specified action.
	//
	// Using interfaces is overkill for now but is the way to make this
	// extensible.  (Delete this comment if we add more than one control)
	switch typedMsg := ctrlMsg.(type) {
	case cbMessage:
		d.registeredCb = typedMsg.cb
	default:
		// We don't know this type, log and ignore
		log.Info("dispatcher control can not handle type %T", typedMsg)
	}
}
