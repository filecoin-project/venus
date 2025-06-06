package dispatcher

import (
	"container/list"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	atomic2 "sync/atomic"
	"time"

	"github.com/filecoin-project/pubsub"
	"github.com/filecoin-project/venus/pkg/chain"
	"github.com/filecoin-project/venus/pkg/chainsync/types"
	types2 "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/streadway/handy/atomic"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("chainsync.dispatcher")

// DefaultInQueueSize is the bucketSize of the channel used for receiving targets from producers.
const DefaultInQueueSize = 5

// DefaultWorkQueueSize is the bucketSize of the work queue
const DefaultWorkQueueSize = 15

const LocalIncoming = "incoming"

// dispatchSyncer is the interface of the logic syncing incoming chains
type dispatchSyncer interface {
	Head() *types2.TipSet
	HandleNewTipSet(context.Context, *types.Target) error
	ValidateMsgMeta(ctx context.Context, fblk *types2.FullBlock) error
	SyncCheckpoint(ctx context.Context, tsk types2.TipSetKey) error
}

// NewDispatcher creates a new syncing dispatcher with default queue sizes.
func NewDispatcher(catchupSyncer dispatchSyncer, chainStore *chain.Store) *Dispatcher {
	return NewDispatcherWithSizes(catchupSyncer, chainStore, DefaultWorkQueueSize, DefaultInQueueSize)
}

// NewDispatcherWithSizes creates a new syncing dispatcher.
func NewDispatcherWithSizes(syncer dispatchSyncer, chainStore *chain.Store, workQueueSize, inQueueSize int) *Dispatcher {
	return &Dispatcher{
		workTracker:     types.NewTargetTracker(workQueueSize),
		syncer:          syncer,
		incoming:        make(chan *types.Target, inQueueSize),
		control:         make(chan interface{}, 1),
		registeredCb:    func(t *types.Target, err error) {},
		cancelControler: list.New(),
		maxCount:        1,
		incomingPubsub:  pubsub.New(50),
		chainStore:      chainStore,
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

	cancelControler *list.List
	lk              sync.Mutex
	conCurrent      atomic.Int
	maxCount        int64

	incomingPubsub *pubsub.PubSub
	chainStore     *chain.Store
}

// SyncTracker returnss the target tracker of syncing
func (d *Dispatcher) SyncTracker() *types.TargetTracker {
	return d.workTracker
}

func (d *Dispatcher) sendHead(ci *types2.ChainInfo) error {
	ctx := context.Background()
	fts := ci.FullTipSet
	if fts == nil {
		return fmt.Errorf("got nil tipset")
	}

	for _, b := range fts.Blocks {
		if err := d.syncer.ValidateMsgMeta(ctx, b); err != nil {
			log.Warnf("invalid block %s received: %s", b.Cid(), err)
			return fmt.Errorf("validate block %s message meta failed: %v", b.Cid(), err)
		}
	}

	for _, b := range fts.Blocks {
		_, err := d.chainStore.PutObject(ctx, b.Header)
		if err != nil {
			return fmt.Errorf("fail to save block to tipset")
		}
	}

	d.incomingPubsub.Pub(fts.TipSet().Blocks(), LocalIncoming)

	return d.addTracker(ci)
}

// SendHello handles chain information from bootstrap peers.
func (d *Dispatcher) SendHello(ci *types2.ChainInfo) error {
	return d.sendHead(ci)
}

// SendOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) SendOwnBlock(ci *types2.ChainInfo) error {
	return d.sendHead(ci)
}

// SendGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) SendGossipBlock(ci *types2.ChainInfo) error {
	return d.sendHead(ci)
}

func (d *Dispatcher) addTracker(ci *types2.ChainInfo) error {
	d.incoming <- &types.Target{
		Head:    ci.FullTipSet.TipSet(),
		Base:    d.syncer.Head(),
		Current: d.syncer.Head(),
		Start:   time.Now(),
		Sender:  ci.Sender,
	}
	return nil
}

// Start launches the business logic for the syncing subsystem.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	go d.processIncoming(syncingCtx)

	go d.syncWorker(syncingCtx)
}

func (d *Dispatcher) processIncoming(ctx context.Context) {
	defer func() {
		log.Info("exiting sync dispatcher")
		if r := recover(); r != nil {
			log.Errorf("panic: %v", r)
			debug.PrintStack()
		}
	}()

	for {
		// Handle shutdown
		select {
		case <-ctx.Done():
			log.Info("context done")
			return
		case ctrl := <-d.control:
			log.Infof("processing control: %v", ctrl)
			d.processCtrl(ctrl)
		case target := <-d.incoming:
			// Sort new targets by putting on work queue.
			if d.workTracker.Add(target) {
				log.Infow("received new tipset", "height", target.Head.Height(), "blocks", target.Head.Len(), "from",
					target.Sender, "current work len", d.workTracker.Len(), "incoming channel len", len(d.incoming))
			}
		}
	}
}

// SetConcurrent set the max goroutine to syncing target
func (d *Dispatcher) SetConcurrent(number int64) {
	d.lk.Lock()
	defer d.lk.Unlock()
	d.maxCount = number
	diff := d.conCurrent.Get() - d.maxCount
	if diff > 0 {
		ele := d.cancelControler.Back()
		for ele != nil && diff > 0 {
			ele.Value.(context.CancelFunc)()
			preEle := ele.Prev()
			d.cancelControler.Remove(ele)
			ele = preEle
			diff--
		}
	}
}

// Concurrent get current max syncing goroutine
func (d *Dispatcher) Concurrent() int64 {
	d.lk.Lock()
	defer d.lk.Unlock()
	return d.maxCount
}

func (d *Dispatcher) IncomingBlocks(ctx context.Context) (<-chan *types2.BlockHeader, error) {
	sub := d.incomingPubsub.Sub(LocalIncoming)
	out := make(chan *types2.BlockHeader, 32)

	go func() {
		defer func() {
			close(out)

			d.incomingPubsub.Unsub(sub)
		}()

		for {
			select {
			case val, ok := <-sub:
				if !ok {
					return
				}
				for _, blk := range val.([]*types2.BlockHeader) {
					select {
					case out <- blk:
					case <-ctx.Done():
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (d *Dispatcher) selectTarget(lastTarget *types.Target, ch <-chan struct{}) (*types.Target, bool) {
exitFor:
	for { // we are purpose to consume all notifies in channel
		select {
		case _, isok := <-ch:
			if !isok {
				return nil, false
			}
		default:
			break exitFor
		}
	}
	return d.workTracker.Select()
}

func (d *Dispatcher) syncWorker(ctx context.Context) {
	defer func() {
		log.Infof("dispatcher.syncworker exit.")
	}()

	const chKey = "sync-worker"
	ch := d.workTracker.SubNewTarget(chKey, 20)
	unsolvedNotify := int64(0)
	var lastTarget *types.Target
	for {
		select {
		// must make sure, 'ch' is not blocked, or may cause syncing problems
		case _, isok := <-ch:
			if !isok {
				break
			}
			if syncTarget, popped := d.selectTarget(lastTarget, ch); popped {
				lastTarget = syncTarget
				if d.conCurrent.Get() < d.maxCount {
					atomic2.StoreInt64(&unsolvedNotify, 0)
					syncTarget.State = types.StateInSyncing
					ctx, cancel := context.WithCancel(ctx)
					d.cancelControler.PushBack(cancel)
					d.conCurrent.Add(1)
					go func() {
						err := d.syncer.HandleNewTipSet(ctx, syncTarget)
						if err != nil {
							log.Infof("failed sync of %v at %d  %s", syncTarget.Head.Key(), syncTarget.Head.Height(), err)
						}
						d.workTracker.Remove(syncTarget)
						d.registeredCb(syncTarget, err)
						d.conCurrent.Add(-1)

						// new 'target' notify may ignored, because of 'conCurrent' reaching 'maxCount',
						// that means there is a new 'target' waiting for solving.
						if atomic2.LoadInt64(&unsolvedNotify) > 0 {
							ch <- struct{}{}
						}
					}()
				} else {
					atomic2.StoreInt64(&unsolvedNotify, 1)
				}
			}
		case <-ctx.Done():
			atomic2.StoreInt64(&unsolvedNotify, 0)
			d.workTracker.UnsubNewTarget(chKey)
			ch = nil
			log.Infof("context.done in dispatcher.syncworker.")
			return
		}
	}
}

// RegisterCallback registers a callback on the dispatcher that
// will fire after every successful target sync.
func (d *Dispatcher) RegisterCallback(cb func(*types.Target, error)) {
	d.control <- cbMessage{cb: cb}
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

func (d *Dispatcher) SyncCheckpoint(ctx context.Context, tsk types2.TipSetKey) error {
	return d.syncer.SyncCheckpoint(ctx, tsk)
}
