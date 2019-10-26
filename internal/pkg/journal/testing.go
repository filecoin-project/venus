package journal

import (
	"sync"
	"testing"
	"time"

	"github.com/filecoin-project/go-filecoin/internal/pkg/clock"
)

// NewInMemoryJournal returns a journal backed by an in-memory map.
func NewInMemoryJournal(t *testing.T, clk clock.Clock) Journal {
	return &MemoryJournal{
		t:      t,
		clock:  clk,
		topics: make(map[string][]entry),
	}
}

// MemoryJournal represents a journal held in memory.
type MemoryJournal struct {
	t        *testing.T
	clock    clock.Clock
	topicsMu sync.Mutex
	topics   map[string][]entry
}

// Topic returns a Writer with the provided `topic`.
func (mj *MemoryJournal) Topic(topic string) Writer {
	mr := &MemoryWriter{
		topic:   topic,
		journal: mj,
	}
	return mr
}

type entry struct {
	time  time.Time
	event string
	kvs   []interface{}
}

// MemoryWriter writes journal entires in memory.
type MemoryWriter struct {
	topic   string
	journal *MemoryJournal
}

// Write records an operation and its metadata to a Journal accepting variadic key-value
// pairs.
func (mw *MemoryWriter) Write(event string, kvs ...interface{}) {
	if len(kvs)%2 != 0 {
		mw.journal.t.Fatalf("journal write call has odd number of key values pairs: %d event: %s topic: %s", len(kvs), event, mw.topic)
	}
	mw.journal.topicsMu.Lock()
	mw.journal.topics[mw.topic] = append(mw.journal.topics[mw.topic], entry{
		event: event,
		time:  mw.journal.clock.Now(),
		kvs:   kvs,
	})
	mw.journal.topicsMu.Unlock()
}
