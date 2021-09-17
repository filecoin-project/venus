package apitypes

import (
	"fmt"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/pkg/types"
	"github.com/ipfs/go-cid"
	"time"
)

type ComputeStateOutput struct {
	Root  cid.Cid
	Trace []*types.InvocResult
}

type SyncState struct {
	ActiveSyncs []ActiveSync

	VMApplied uint64
}

//just compatible code lotus
type SyncStateStage int

const (
	StageIdle = SyncStateStage(iota)
	StageHeaders
	StagePersistHeaders
	StageMessages
	StageSyncComplete
	StageSyncErrored
	StageFetchingMessages
)

func (v SyncStateStage) String() string {
	switch v {
	case StageHeaders:
		return "header sync"
	case StagePersistHeaders:
		return "persisting headers"
	case StageMessages:
		return "message sync"
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

type ActiveSync struct {
	WorkerID uint64
	Base     *types.TipSet
	Target   *types.TipSet

	Stage  SyncStateStage
	Height abi.ChainEpoch

	Start   time.Time
	End     time.Time
	Message string
}
