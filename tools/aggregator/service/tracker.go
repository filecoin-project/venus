package aggregator

import (
	"fmt"
	"sync"
)

// Tracker tracks node consensus from heartbeats
type Tracker struct {
	// NodeTips is a mapping from peerID's to Tipsets
	NodeTips map[string]string
	// TipsCount is a mapping from tipsets to number of peers mining on said tipset.
	TipsCount map[string]int
	// TrackedNodes is the set of nodes currently connected to the aggregator, this
	// value is updated using the net.NotifyBundle in service_utils.go
	TrackedNodes map[string]struct{}

	// mutex that protects access to the fields in Tracker:
	// - NodeTips
	// - TipsCount
	// - TrackedNodes
	mux sync.Mutex
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
	return &Tracker{
		TrackedNodes: make(map[string]struct{}),
		NodeTips:     make(map[string]string),
		TipsCount:    make(map[string]int),
	}
}

// ConnectNode will add a node to the trackers `TrackedNode` set and
// increment the connected_nodes prometheus metric.
func (t *Tracker) ConnectNode(peer string) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.TrackedNodes[peer] = struct{}{}
	connectedNodes.WithLabelValues(aggregatorLabel).Inc()
}

// DisconnectNode will remove a node from the trackers `TrackedNode` set and
// decrement the connected_nodes prometheus metric.
func (t *Tracker) DisconnectNode(peer string) {
	t.mux.Lock()
	defer t.mux.Unlock()

	delete(t.TrackedNodes, peer)
	curTs := t.NodeTips[peer]
	t.TipsCount[curTs]--
	if t.TipsCount[curTs] == 0 {
		delete(t.TipsCount, curTs)
	}
	delete(t.NodeTips, peer)
	connectedNodes.WithLabelValues(aggregatorLabel).Dec()
}

// TrackConsensus updates the metrics Tracker keeps, threadsafe
func (t *Tracker) TrackConsensus(peer, ts string) {
	log.Debugf("track peer: %s, tipset: %s", peer, ts)
	t.mux.Lock()
	defer t.mux.Unlock()

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
	tn := len(t.TrackedNodes)
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
