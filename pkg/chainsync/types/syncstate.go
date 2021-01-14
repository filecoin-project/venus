package types

import "fmt"

//just compatible code lotus
type SyncStateStage int

const (
	StageIdle = SyncStateStage(iota)
	StateInSyncing
	StageSyncComplete
	StageSyncErrored
	StageFetchingMessages
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
	case StageFetchingMessages:
		return "fetching messages"
	default:
		return fmt.Sprintf("<unknown: %d>", v)
	}
}
