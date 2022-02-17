package market

// DagstoreShardInfo is the serialized form of dagstore.DagstoreShardInfo that
// we expose through JSON-RPC to avoid clients having to depend on the
// dagstore lib.
type DagstoreShardInfo struct {
	Key   string
	State string
	Error string
}

// DagstoreShardResult enumerates results per shard.
type DagstoreShardResult struct {
	Key     string
	Success bool
	Error   string
}

type DagstoreInitializeAllParams struct {
	MaxConcurrency int
	IncludeSealed  bool
}

// DagstoreInitializeAllEvent represents an initialization event.
type DagstoreInitializeAllEvent struct {
	Key     string
	Event   string // "start", "end"
	Success bool
	Error   string
	Total   int
	Current int
}
