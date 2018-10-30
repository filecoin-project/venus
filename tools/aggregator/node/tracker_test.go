package aggregator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTipsetForTestGetter() func() string {
	i := 0
	return func() string {
		s := fmt.Sprintf("tipset%d", i)
		i++
		return s
	}
}

func TestTrackTipset(t *testing.T) {
	assert := assert.New(t)
	t.Run("simple test", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts0 := mkTs()
		// 3 nodes all on the same tipset
		tracker.TrackConsensus("peer0", ts0)
		assert.Equal(1, tracker.TrackerSummary().TrackedNodes)
		tracker.TrackConsensus("peer1", ts0)
		assert.Equal(2, tracker.TrackerSummary().TrackedNodes)
		tracker.TrackConsensus("peer2", ts0)
		assert.Equal(3, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(3, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(0, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts0, tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

		// move node 1 ahead
		ts1 := mkTs()
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts0)
		tracker.TrackConsensus("peer2", ts0)
		assert.Equal(3, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(2, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(1, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts0, tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

		// repeat should be it idempotent
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts0)
		tracker.TrackConsensus("peer2", ts0)
		assert.Equal(3, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(2, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(1, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts0, tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

		// nodes 1 and 2 catch node 0
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts1)
		assert.Equal(3, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(3, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(0, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts1, tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)
	})

	t.Run("test no consensus, all different tipsets", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		// 3 nodes all on different tipset
		tracker.TrackConsensus("peer0", mkTs())
		tracker.TrackConsensus("peer1", mkTs())
		tracker.TrackConsensus("peer2", mkTs())
		assert.Equal(3, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(0, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(3, tracker.TrackerSummary().NodesInDispute)
		assert.Equal("", tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

	})
	t.Run("test no consensus, split tipset", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts1 := mkTs()
		ts2 := mkTs()
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts2)
		tracker.TrackConsensus("peer3", ts2)
		assert.Equal(4, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(0, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(4, tracker.TrackerSummary().NodesInDispute)
		assert.Equal("", tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

	})
	t.Run("test no consensus, split tipset and a spare", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts1 := mkTs()
		ts2 := mkTs()
		ts3 := mkTs()
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts2)
		tracker.TrackConsensus("peer3", ts2)
		tracker.TrackConsensus("peer4", ts3)
		assert.Equal(5, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(0, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(5, tracker.TrackerSummary().NodesInDispute)
		assert.Equal("", tracker.TrackerSummary().HeaviestTipset)
		logStats(t, tracker)

	})
	t.Run("test no consensus, 2 majority 3 minority", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts1 := mkTs()
		ts2 := mkTs()
		ts3 := mkTs()
		ts4 := mkTs()
		ts5 := mkTs()
		// maj 1
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts1)
		// maj 2
		tracker.TrackConsensus("peer3", ts2)
		tracker.TrackConsensus("peer4", ts2)
		tracker.TrackConsensus("peer5", ts2)
		// min 1
		tracker.TrackConsensus("peer6", ts3)
		tracker.TrackConsensus("peer7", ts3)
		// min 2
		tracker.TrackConsensus("peer8", ts4)
		tracker.TrackConsensus("peer9", ts4)
		// min 3
		tracker.TrackConsensus("peer10", ts5)
		assert.Equal(11, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(0, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(11, tracker.TrackerSummary().NodesInDispute)
		assert.Equal("", tracker.TrackerSummary().HeaviestTipset)

	})
	t.Run("test consensus, 1 majority 1 minority", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts1 := mkTs()
		ts2 := mkTs()
		// maj
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts1)
		// min
		tracker.TrackConsensus("peer3", ts2)
		tracker.TrackConsensus("peer4", ts2)
		assert.Equal(5, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(3, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(2, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts1, tracker.TrackerSummary().HeaviestTipset)

	})
	t.Run("test consensus, 1 majority 3 minority", func(t *testing.T) {
		mkTs := newTipsetForTestGetter()
		tracker := NewTracker()
		logStats(t, tracker)

		ts1 := mkTs()
		ts2 := mkTs()
		ts3 := mkTs()
		ts4 := mkTs()
		// maj
		tracker.TrackConsensus("peer0", ts1)
		tracker.TrackConsensus("peer1", ts1)
		tracker.TrackConsensus("peer2", ts1)
		// min 1
		tracker.TrackConsensus("peer3", ts2)
		tracker.TrackConsensus("peer4", ts2)
		// min 2
		tracker.TrackConsensus("peer5", ts3)
		tracker.TrackConsensus("peer6", ts3)
		// min 3
		tracker.TrackConsensus("peer7", ts4)
		assert.Equal(8, tracker.TrackerSummary().TrackedNodes)
		assert.Equal(3, tracker.TrackerSummary().NodesInConsensus)
		assert.Equal(5, tracker.TrackerSummary().NodesInDispute)
		assert.Equal(ts1, tracker.TrackerSummary().HeaviestTipset)
	})
}

// run like `go test -v -run TestTrackTipset` for logging
func logStats(t *testing.T, tracker *Tracker) {
	t.Logf("TipsCount: %v", tracker.TipsCount)
	t.Logf("NodeToTips: %v", tracker.NodeTips)
	t.Log("----------------")
}
