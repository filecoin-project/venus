package aggregator

import (
	"fmt"
	"sort"
	"sync"
)

// Tracker tracks node consensus from heartbeats
type Tracker struct {
	// NodeTips tracks what tipset a node is on
	NodeTips map[string]string
	// TipsCount tacks how many nodes are at each tipset
	TipsCount map[string]int

	// TODO replace with value calculated from net/BundleNotify callbacks
	trackedNodes map[string]struct{}
	// Protects maps
	mux sync.Mutex
}

type tipsetRank struct {
	Tipset string
	Rank   int
}

// TrackerSummary represents the information a tracker has on the nodes
// its receiving heartbeats from
type TrackerSummary struct {
	TrackedNodes     int
	NodesInConsensus int
	NodesInDispute   int
	HeaviestTipset   string
}

// NewTracker initializes a tracker
func NewTracker() *Tracker {
	// TODO add a context here else you can't shutdown all nice and purdey
	return &Tracker{
		trackedNodes: make(map[string]struct{}),
		NodeTips:     make(map[string]string),
		TipsCount:    make(map[string]int),
	}
}

// TrackConsensus updates the metrics Tracker keeps, threadsafe
func (t *Tracker) TrackConsensus(peer, ts string) {
	log.Debugf("track peer: %s, tipset: %s", peer, ts)
	t.mux.Lock()
	defer t.mux.Unlock()

	t.trackedNodes[peer] = struct{}{}
	// get the tipset the nodes is con currently
	curTs, ok := t.NodeTips[peer]
	if ok {
		log.Debugf("peer: %s, current tipset: %s", peer, curTs)
		t.TipsCount[curTs]--
		if t.TipsCount[curTs] == 0 {
			delete(t.TipsCount, curTs)
		}
	}

	t.NodeTips[peer] = ts
	t.TipsCount[ts]++
	log.Debugf("update peer: %s, tipset: %s, nodes at tipset: %d", peer, ts, t.TipsCount[ts])
}

// TrackerSummary generates a summary of the metrics Tracker keeps, threadsafe
func (t *Tracker) TrackerSummary() TrackerSummary {
	t.mux.Lock()
	defer t.mux.Unlock()
	tn := len(t.trackedNodes)
	nc, ht := nodesInConsensus(t.TipsCount)
	nd := tn - nc

	nodesConsensus.WithLabelValues(aggregatorLabel).Set(float64(nc))
	nodesDispute.WithLabelValues(aggregatorLabel).Set(float64(nd))
	return TrackerSummary{
		TrackedNodes:     tn,
		NodesInConsensus: nc,
		NodesInDispute:   nd,
		HeaviestTipset:   ht,
	}
}

func (t *Tracker) String() string {
	ts := t.TrackerSummary()
	return fmt.Sprintf("Tracked Nodes: %d, In Consensus: %d, In Dispute: %d, HeaviestTipset: %s", ts.TrackedNodes, ts.NodesInConsensus, ts.NodesInDispute, ts.HeaviestTipset)
}

// nodesInConsensus calculates the number of nodes in consunsus and the heaviesttipset
func nodesInConsensus(tipsetCount map[string]int) (int, string) {
	var out []tipsetRank
	for t, r := range tipsetCount {
		tr := tipsetRank{
			Tipset: t,
			Rank:   r,
		}
		out = append(out, tr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rank > out[j].Rank })
	if len(out) > 1 && out[0].Rank == out[1].Rank {
		return 0, ""
	}
	return out[0].Rank, out[0].Tipset
}
