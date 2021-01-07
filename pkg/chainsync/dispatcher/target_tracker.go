package dispatcher

import (
	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/block"
	"github.com/ipfs/go-cid"
	"sort"
	"sync"
)

// Target tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs.
type Target struct {
	block.ChainInfo
	InSyncing bool
}

// TargetTracker orders dispatcher syncRequests by the underlying `TargetBuckets`'s
// prioritization policy.
//
// It also filters the `TargetBuckets` so that it always contains targets with
// unique chain heads.
//
// It wraps the `TargetBuckets` to prevent panics during
// normal operation.
type TargetTracker struct {
	size      int
	q         TargetBuckets
	targetSet map[string]*Target
	lowWeight fbig.Int
	lk        sync.Mutex
}

// NewTargetTracker returns a new target queue.
func NewTargetTracker(size int) *TargetTracker {
	return &TargetTracker{
		size:      size,
		q:         make(TargetBuckets, 0),
		targetSet: make(map[string]*Target),
		lk:        sync.Mutex{},
		lowWeight: fbig.NewInt(0),
	}
}

// Add adds a sync target to the target queue.
func (tq *TargetTracker) Add(t *Target) {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	//do not sync less weight
	if t.Head.At(0).ParentWeight.LessThan(tq.lowWeight) {
		return
	}

	t, ok := tq.widen(t)
	if !ok {
		return
	}
	if len(tq.q) <= tq.size {
		tq.q = append(tq.q, t)
	} else {
		last := tq.q[len(tq.q)-1]
		delete(tq.targetSet, last.ChainInfo.Head.String())
		tq.q[len(tq.q)-1] = t
	}
	tq.targetSet[t.ChainInfo.Head.String()] = t
	sort.Slice(tq.q, func(i, j int) bool {
		weightI, _ := tq.q[i].Head.ParentWeight()
		weightJ, _ := tq.q[j].Head.ParentWeight()
		return weightI.GreaterThan(weightJ)
	})
	//update lowweight
	tq.lowWeight = tq.q[len(tq.q)-1].Head.At(0).ParentWeight
	return
}

func (tq *TargetTracker) widen(t *Target) (*Target, bool) {
	if len(tq.targetSet) == 0 {
		return t, true
	}

	var err error
	// If already in queue drop quickly
	for _, val := range tq.targetSet {
		if val.Head.Key().ContainsAll(t.Head.Key()) {
			return nil, false
		}
	}

	inWeight, _ := t.Head.ParentWeight()
	sameWeightBlks := make(map[cid.Cid]*block.Block)
	for _, val := range tq.targetSet {
		weight, _ := val.Head.ParentWeight()
		if inWeight.Equals(weight) &&
			val.Head.EnsureHeight() == t.Head.EnsureHeight() &&
			val.Head.EnsureParents() == t.Head.EnsureParents() {
			for _, blk := range val.Head.Blocks() {
				bid := blk.Cid()
				if !t.Head.Key().Has(bid) {
					if _, ok := sameWeightBlks[bid]; !ok {
						sameWeightBlks[bid] = blk
					}
				}
			}
		}
	}

	if len(sameWeightBlks) == 0 {
		return t, true
	}

	blks := t.Head.Blocks()
	for _, blk := range sameWeightBlks {
		blks = append(blks, blk)
	}

	newHead, err := block.NewTipSet(blks...)
	if err != nil {
		return nil, false
	}
	t.Head = newHead
	return t, true
}

// Pop removes and returns the highest priority syncing target. If there is
// nothing in the queue the second argument returns false
func (tq *TargetTracker) Select() (*Target, bool) {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	if tq.q.Len() == 0 {
		return nil, false
	}
	var toSyncTarget *Target
	for _, target := range tq.q {
		if !target.InSyncing {
			toSyncTarget = target
			break
		}
	}

	if toSyncTarget == nil {
		return nil, false
	}
	toSyncTarget.InSyncing = true
	return toSyncTarget, true
}

func (tq *TargetTracker) Remove(t *Target) {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	for index, target := range tq.q {
		if t == target {
			tq.q = append(tq.q[:index], tq.q[index+1:]...)
			break
		}
	}
	popKey := t.ChainInfo.Head.String()
	delete(tq.targetSet, popKey)
}

// Len returns the number of targets in the queue.
func (tq *TargetTracker) Len() int {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	return tq.q.Len()
}

// Buckets returns the number of targets in the queue.
func (tq *TargetTracker) Buckets() TargetBuckets {
	return tq.q
}

// TargetBuckets orders targets by a policy.
//
// The current simple policy is to order syncing requests by claimed chain
// height.
//
// `TargetBuckets` can panic so it shouldn't be used unwrapped
type TargetBuckets []*Target

// Heavily inspired by https://golang.org/pkg/container/heap/
func (rq TargetBuckets) Len() int { return len(rq) }

func (rq TargetBuckets) Less(i, j int) bool {
	// We want Pop to give us the weight priority so we use greater than
	weightI, _ := rq[i].Head.ParentWeight()
	weightJ, _ := rq[j].Head.ParentWeight()
	return weightI.GreaterThan(weightJ)
}

func (rq TargetBuckets) Swap(i, j int) {
	rq[i], rq[j] = rq[j], rq[i]
}

func (rq *TargetBuckets) Pop() interface{} {
	old := *rq
	n := len(old)
	item := old[n-1]
	*rq = old[0 : n-1]
	return item
}
