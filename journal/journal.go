package journal

// Writer defines an interface for recording events and their metadata
type Writer interface {
	// Write records an operation and its metadata to a Journal accepting variadic key-value
	// pairs.
	Write(event string, kvs ...interface{})
}

// Journal defines an interface for creating Journals with a topic.
type Journal interface {
	// Topic returns a Writer that records events for a topic.
	Topic(topic string) Writer
}
