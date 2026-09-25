package types

import "fmt"

// SyncStateStage is the internal sync stage tracked by chainsync; the syncer API
// maps it onto the API-facing types.SyncStateStage in convertSyncStateStage.
type SyncStateStage int

const (
	StageIdle = SyncStateStage(iota)
	StateInSyncing
	StageSyncComplete
	StageSyncErrored
)

func (v SyncStateStage) String() string {
	switch v {
	case StageIdle:
		return "wait"
	case StateInSyncing:
		return "syncing"
	case StageSyncComplete:
		return "complete"
	case StageSyncErrored:
		return "error"
	default:
		return fmt.Sprintf("<unknown: %d>", v)
	}
}
