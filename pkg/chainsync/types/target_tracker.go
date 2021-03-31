package types

import (
	"container/list"
	"sort"
	"strconv"
	"sync"
	"time"

	fbig "github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("chainsync.target")

// Target tracks a logical request of the syncing subsystem to run a
// syncing job against given inputs.
type Target struct {
	State   SyncStateStage
	Base    *types.TipSet
	Current *types.TipSet
	Start   time.Time
	End     time.Time
	Err     error
	types.ChainInfo
}

func (target *Target) IsNeibor(t *Target) bool {
	if target.Head.Height() != t.Head.Height() {
		return false
	}

	targetWeight := t.Head.ParentWeight()
	weightIn := target.Head.ParentWeight()
	if !targetWeight.Equals(weightIn) {
		return false
	}

	if !target.Head.Parents().Equals(t.Head.Parents()) {
		return false
	}
	return true
}

func (target *Target) HasChild(t *Target) bool {
	return target.Head.Key().ContainsAll(t.Head.Key())
}

func (target *Target) Key() string {
	weightIn := target.Head.ParentWeight()
	return weightIn.String() +
		strconv.FormatInt(int64(target.Head.Height()), 10) +
		target.Head.Parents().String()

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
	bucketSize  int
	historySize int
	q           TargetBuckets
	history     *list.List
	targetSet   map[string]*Target
	lowWeight   fbig.Int
	lk          sync.Mutex
}

// NewTargetTracker returns a new target queue.
func NewTargetTracker(size int) *TargetTracker {
	return &TargetTracker{
		bucketSize:  size,
		historySize: 10,
		history:     list.New(),
		q:           make(TargetBuckets, 0),
		targetSet:   make(map[string]*Target),
		lk:          sync.Mutex{},
		lowWeight:   fbig.NewInt(0),
	}
}

// Add adds a sync target to the target queue.
func (tq *TargetTracker) Add(t *Target) bool {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	//do not sync less weight
	if t.Head.At(0).ParentWeight.LessThan(tq.lowWeight) {
		return false
	}

	t, ok := tq.widen(t)
	if !ok {
		return false
	}

	//replace last idle task because of less weight
	var replaceIndex int
	var replaceTarget *Target
	//try to replace a idea child target
	for i := len(tq.q) - 1; i > -1; i-- {
		if t.HasChild(tq.q[i]) && tq.q[i].State == StageIdle {
			replaceTarget = tq.q[i]
			replaceIndex = i
			log.Info("%s replace a child target at %d", t.Head.String(), i)
			break
		}
	}

	if replaceTarget == nil {
		//replace a least weight idle
		for i := len(tq.q) - 1; i > -1; i-- {
			if tq.q[i].State == StageIdle {
				replaceTarget = tq.q[i]
				replaceIndex = i
				log.Info("%s replace a idle target at %d", t.Head.String(), i)
				break
			}
		}
	}

	if replaceTarget == nil {
		if len(tq.q) <= tq.bucketSize {
			tq.q = append(tq.q, t)
		} else {
			return false
		}
	} else {
		delete(tq.targetSet, replaceTarget.ChainInfo.Head.String())
		tq.q[replaceIndex] = t
	}

	tq.targetSet[t.ChainInfo.Head.String()] = t
	sortTarget(tq.q)
	//update lowweight
	tq.lowWeight = tq.q[len(tq.q)-1].Head.At(0).ParentWeight
	return true
}

//sort by weight and than sort by block number in ts
func sortTarget(target TargetBuckets) {
	//group key
	groups := make(map[string][]*Target)
	var keys []fbig.Int
	for _, t := range target {
		weight := t.Head.ParentWeight()
		if _, ok := groups[weight.String()]; ok {
			groups[weight.String()] = append(groups[weight.String()], t)
		} else {
			groups[weight.String()] = []*Target{t}
			keys = append(keys, weight)
		}
	}

	//sort group
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].GreaterThan(keys[j])
	})

	for _, key := range keys {
		inGroup := groups[key.String()]
		sort.Slice(inGroup, func(i, j int) bool {
			return inGroup[i].Head.Len() > inGroup[j].Head.Len()
		})
	}

	//sort
	count := 0
	for _, key := range keys {
		for _, t := range groups[key.String()] {
			target[count] = t
			count++
		}
	}
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

	sameWeightBlks := make(map[cid.Cid]*types.BlockHeader)
	for _, val := range tq.targetSet {
		if val.IsNeibor(t) {
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

	newHead, err := types.NewTipSet(blks...)
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
		if target.State == StageIdle {
			toSyncTarget = target
			break
		}
	}

	if toSyncTarget == nil {
		return nil, false
	}
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
	t.End = time.Now()
	if tq.history.Len() > tq.historySize {
		tq.history.Remove(tq.history.Front()) //remove olddest
		popKey := tq.history.Front().Value.(*Target).ChainInfo.Head.String()
		delete(tq.targetSet, popKey)
	}
	tq.history.PushBack(t)
}

func (tq *TargetTracker) History() []*Target {
	tq.lk.Lock()
	defer tq.lk.Unlock()
	var targets []*Target
	for target := tq.history.Front(); target != nil; target = target.Next() {
		targets = append(targets, target.Value.(*Target))
	}
	return targets
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
	weightI := rq[i].Head.ParentWeight()
	weightJ := rq[j].Head.ParentWeight()
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
