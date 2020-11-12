package dispatcher

import (
	"container/heap"
	"context"
	"fmt"
	"runtime/debug"

	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/venus/internal/pkg/block"
	"github.com/filecoin-project/venus/internal/pkg/util/moresync"
)

var log = logging.Logger("chainsync.dispatcher")

// DefaultInQueueSize is the size of the channel used for receiving targets from producers.
const DefaultInQueueSize = 5

// DefaultWorkQueueSize is the size of the work queue
const DefaultWorkQueueSize = 20

// MaxEpochGap is the maximum number of epochs chainsync can fall behind
// before catching up
const MaxEpochGap = 10

// dispatchSyncer is the interface of the logic syncing incoming chains
type dispatchSyncer interface {
	HandleNewTipSet(context.Context, *block.ChainInfo, bool) error
}

type transitionSyncer interface {
	SetStagedHead(context.Context) error
}

// chainHeadState is the interface for determining the head of the chain
type chainHeadState interface {
	GetHead() block.TipSetKey
	GetTipSet(block.TipSetKey) (*block.TipSet, error)
}

// NewDispatcher creates a new syncing dispatcher with default queue sizes.
func NewDispatcher(catchupSyncer dispatchSyncer, trans Transitioner) *Dispatcher {
	return NewDispatcherWithSizes(catchupSyncer, trans, DefaultWorkQueueSize, DefaultInQueueSize)
}

// NewDispatcherWithSizes creates a new syncing dispatcher.
func NewDispatcherWithSizes(syncer dispatchSyncer, trans Transitioner, workQueueSize, inQueueSize int) *Dispatcher {
	return &Dispatcher{
		workQueue:     NewTargetQueue(),
		workQueueSize: workQueueSize,
		syncer:        syncer,
		transitioner:  trans,
		incoming:      make(chan Target, inQueueSize),
		control:       make(chan interface{}, 1),
		registeredCb:  func(t Target, err error) {},
	}
}

// cbMessage registers a user callback to be fired following every successful
// sync.
type cbMessage struct {
	cb func(Target, error)
}

// Dispatcher receives, sorts and dispatches targets to the catchupSyncer to control
// chain syncing.
//
// New targets arrive over the incoming channel. The dispatcher then puts them
// into the workQueue which sorts them by their claimed chain height. The
// dispatcher pops the highest priority target from the queue and then attempts
// to sync the target using its internal catchupSyncer.
//
// The dispatcher has a simple control channel. It reads this for external
// controls. Currently there is only one kind of control message.  It registers
// a callback that the dispatcher will call after every non-erroring sync.
type Dispatcher struct {
	// workQueue is a priority queue of target chain heads that should be
	// synced
	workQueue     *TargetQueue
	workQueueSize int
	// incoming is the queue of incoming sync targets to the dispatcher.
	incoming chan Target
	// syncer is used for dispatching sync targets for chain heads to sync
	// local chain state to these targets.
	syncer dispatchSyncer

	// catchup is true when the syncer is in catchup mode
	catchup bool
	// transitioner wraps logic for transitioning between catchup and follow states.
	transitioner Transitioner

	// registeredCb is a callback registered over the control channel.  It
	// is called after every successful sync.
	registeredCb func(Target, error)
	// control is a queue of control messages not yet processed.
	control chan interface{}

	// syncTargetCount counts the number of successful syncs.
	syncTargetCount uint64
}

// SendHello handles chain information from bootstrap peers.
func (d *Dispatcher) SendHello(ci *block.ChainInfo) error {
	return d.enqueue(ci)
}

// SendOwnBlock handles chain info from a node's own mining system
func (d *Dispatcher) SendOwnBlock(ci *block.ChainInfo) error {
	return d.enqueue(ci)
}

// SendGossipBlock handles chain info from new blocks sent on pubsub
func (d *Dispatcher) SendGossipBlock(ci *block.ChainInfo) error {
	return d.enqueue(ci)
}

func (d *Dispatcher) enqueue(ci *block.ChainInfo) error {
	d.incoming <- Target{ChainInfo: *ci}
	return nil
}

// Start launches the business logic for the syncing subsystem.
func (d *Dispatcher) Start(syncingCtx context.Context) {
	go func() {
		defer func() {
			log.Errorf("exiting")
			if r := recover(); r != nil {
				log.Errorf("panic: %v", r)
				debug.PrintStack()
			}
		}()

		var last *Target
		for {
			// Handle shutdown
			select {
			case <-syncingCtx.Done():
				log.Infof("context done")
				return
			default:
			}

			// Handle control signals
			select {
			case ctrl := <-d.control:
				log.Infof("processing control: %v", ctrl)
				d.processCtrl(ctrl)
			default:
			}

			// Handle incoming targets
			var ws []Target
			if last != nil {
				ws = append(ws, *last)
				last = nil
			}
			select {
			case first := <-d.incoming:
				ws = append(ws, first)
				ws = append(ws, d.drainIncoming()...)
				log.Infof("received %v incoming targets: %v", len(ws), first)
			default:
			}
			catchup, err := d.transitioner.MaybeTransitionToCatchup(d.catchup, ws)
			if err != nil {
				log.Errorf("state update error from reading chain head %s", err)
			} else {
				d.catchup = catchup
			}
			log.Infof("catchup: %v", catchup)
			for i, syncTarget := range ws {
				// Drop targets we don't have room for
				if d.workQueue.Len() >= d.workQueueSize {
					log.Warnf("not enough space for %d targets on work queue", len(ws)-i)
					break
				}
				// Sort new targets by putting on work queue.
				d.workQueue.Push(syncTarget)
			}

			log.Infof("workQueue len: %v", d.workQueue.Len())

			// Check for work to do
			log.Infof("processing work queue of %d", d.workQueue.Len())
			syncTarget, popped := d.workQueue.Pop()
			if popped {
				log.Infof("processing %v", syncTarget)
				// Do work
				err := d.syncer.HandleNewTipSet(syncingCtx, &syncTarget.ChainInfo, d.catchup)
				log.Infof("finished processing %v", syncTarget)
				if err != nil {
					fmt.Printf("failed sync of %v (catchup=%t): %s\n", &syncTarget.ChainInfo, d.catchup, err)
					log.Infof("failed sync of %v (catchup=%t): %s", &syncTarget.ChainInfo, d.catchup, err)
				}
				d.syncTargetCount++
				d.registeredCb(syncTarget, err)
				follow, err := d.transitioner.MaybeTransitionToFollow(syncingCtx, d.catchup, d.workQueue.Len())
				if err != nil {
					log.Errorf("state update error setting head %s", err)
				} else {
					d.catchup = !follow
					log.Infof("catchup state: %v", d.catchup)
				}
			} else {
				// No work left, block until something shows up
				log.Infof("drained work queue, waiting")
				select {
				case extra := <-d.incoming:
					log.Debugf("stopped waiting, received %v", extra)
					last = &extra
				}
			}
		}
	}()
}

func (d *Dispatcher) drainIncoming() []Target {
	// drainProduced reads all values within the incoming channel buffer at time
	// of calling without blocking.  It reads at most incomingBufferSize.
	//
	// Note: this relies on a single reader of the incoming channel to
	// avoid blocking.
	n := len(d.incoming)
	produced := make([]Target, n)
	for i := 0; i < n; i++ {
		produced[i] = <-d.incoming
	}
	return produced
}

// RegisterCallback registers a callback on the dispatcher that
// will fire after every successful target sync.
func (d *Dispatcher) RegisterCallback(cb func(Target, error)) {
	d.control <- cbMessage{cb: cb}
}

// WaiterForTarget returns a function that will block until the dispatcher
// processes the given target and returns the error produced by that targer
func (d *Dispatcher) WaiterForTarget(waitKey block.TipSetKey) func() error {
	processed := moresync.NewLatch(1)
	var syncErr error
	d.RegisterCallback(func(t Target, err error) {
		if t.ChainInfo.Head.Equals(waitKey) {
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

// Target tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs.
type Target struct {
	block.ChainInfo
}

// Transitioner determines whether the caller should move between catchup and
// follow states.
type Transitioner interface {
	MaybeTransitionToCatchup(bool, []Target) (bool, error)
	MaybeTransitionToFollow(context.Context, bool, int) (bool, error)
	TransitionChannel() chan bool
}

// GapTransitioner changes state based on the detection of gaps between the
// local head and syncing targets.
type GapTransitioner struct {
	// headState is used to determine the head tipset height for switching
	// measuring gaps.
	headState chainHeadState
	// headSetter sets the chain head to the internal staged value.
	headSetter transitionSyncer
	// transitionCh emits true when transitioning to catchup and false
	// when transitioning to follow
	// 挖矿开关: 追赶链时暂停挖矿,跟随链时启动挖矿
	transitionCh chan bool
}

// NewGapTransitioner returns a new gap transitioner
func NewGapTransitioner(headState chainHeadState, headSetter transitionSyncer) *GapTransitioner {
	return &GapTransitioner{
		headState:    headState,
		headSetter:   headSetter,
		transitionCh: make(chan bool, 0),
	}
}

// MaybeTransitionToCatchup returns true if the state is already catchup, or if
// it should transition from follow to catchup. Undefined on error.
func (gt *GapTransitioner) MaybeTransitionToCatchup(inCatchup bool, targets []Target) (bool, error) {
	if inCatchup {
		return true, nil
	}
	// current head height
	head, err := gt.headState.GetTipSet(gt.headState.GetHead())
	if err != nil {
		return false, err
	}
	headHeight, err := head.Height()
	if err != nil {
		return false, err
	}

	// transition from follow to catchup if incoming targets have gaps
	// Note: we run this check even on targets we may drop
	for _, target := range targets {
		if target.Height > headHeight+MaxEpochGap {
			// gt.transitionCh <- true
			return true, nil
		}
	}
	return false, nil
}

// MaybeTransitionToFollow returns true if the state is already follow, or if
// it should transition from catchup to follow.  Undefined on error.
func (gt *GapTransitioner) MaybeTransitionToFollow(ctx context.Context, inCatchup bool, outstandingTargets int) (bool, error) {
	if !inCatchup {
		return true, nil
	}

	// transition from catchup to follow if the work queue is empty.
	// this is safe -- all gap conditions cause syncing to enter catchup
	// this is pessimistic -- gap conditions could be gone before we transition
	if outstandingTargets == 0 {
		// gt.transitionCh <- false
		// set staging to head on transition catchup --> follow
		return true, gt.headSetter.SetStagedHead(ctx)
	}

	return false, nil
}

// TransitionChannel returns a channel emitting transition flags.
func (gt *GapTransitioner) TransitionChannel() chan bool {
	return gt.transitionCh
}

// TargetQueue orders dispatcher syncRequests by the underlying `targetQueue`'s
// prioritization policy.
//
// It also filters the `targetQueue` so that it always contains targets with
// unique chain heads.
//
// It wraps the `targetQueue` to prevent panics during
// normal operation.
type TargetQueue struct {
	q         targetQueue
	targetSet map[string]struct{}
}

// NewTargetQueue returns a new target queue.
func NewTargetQueue() *TargetQueue {
	rq := make(targetQueue, 0)
	heap.Init(&rq)
	return &TargetQueue{
		q:         rq,
		targetSet: make(map[string]struct{}),
	}
}

// Push adds a sync target to the target queue.
func (tq *TargetQueue) Push(t Target) {
	// If already in queue drop quickly
	if _, inQ := tq.targetSet[t.ChainInfo.Head.String()]; inQ {
		return
	}
	heap.Push(&tq.q, t)
	tq.targetSet[t.ChainInfo.Head.String()] = struct{}{}
	return
}

// Pop removes and returns the highest priority syncing target. If there is
// nothing in the queue the second argument returns false
func (tq *TargetQueue) Pop() (Target, bool) {
	if tq.Len() == 0 {
		return Target{}, false
	}
	req := heap.Pop(&tq.q).(Target)
	popKey := req.ChainInfo.Head.String()
	delete(tq.targetSet, popKey)
	return req, true
}

// Len returns the number of targets in the queue.
func (tq *TargetQueue) Len() int {
	return tq.q.Len()
}

// targetQueue orders targets by a policy.
//
// The current simple policy is to order syncing requests by claimed chain
// height.
//
// `targetQueue` can panic so it shouldn't be used unwrapped
type targetQueue []Target

// Heavily inspired by https://golang.org/pkg/container/heap/
func (rq targetQueue) Len() int { return len(rq) }

func (rq targetQueue) Less(i, j int) bool {
	// We want Pop to give us the highest priority so we use greater than
	return rq[i].Height > rq[j].Height
}

func (rq targetQueue) Swap(i, j int) {
	rq[i], rq[j] = rq[j], rq[i]
}

func (rq *targetQueue) Push(x interface{}) {
	syncReq := x.(Target)
	*rq = append(*rq, syncReq)
}

func (rq *targetQueue) Pop() interface{} {
	old := *rq
	n := len(old)
	item := old[n-1]
	*rq = old[0 : n-1]
	return item
}
