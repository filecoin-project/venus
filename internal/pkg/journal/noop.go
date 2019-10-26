package journal

// NewNoopJournal returns a NoopJournal.
func NewNoopJournal() Journal { return &NoopJournal{} }

// NoopJournal implements the Journal interface and Noops for all operations.
type NoopJournal struct{}

// Topic returns a NoopWriter.
func (nj *NoopJournal) Topic(topic string) Writer { return &NoopWriter{} }

// NoopWriter implements the Writer interface and Noops for all operations.
type NoopWriter struct{}

// Write Noops for all operations.
func (nw *NoopWriter) Write(event string, kvs ...interface{}) {}
