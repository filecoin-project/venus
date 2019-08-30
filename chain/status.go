package chain

import (
	"fmt"
	"sync"

	"github.com/filecoin-project/go-filecoin/types"
)

// Reporter defines an interface to updating and reporting the status of the blockchain.
type Reporter interface {
	UpdateStatus(...StatusUpdates)
	Status() Status
}

// StatusReporter implements the Reporter interface.
type StatusReporter struct {
	statusMu sync.Mutex
	status   *Status
}

// UpdateStatus updates the status heald by StatusReporter.
func (sr *StatusReporter) UpdateStatus(update ...StatusUpdates) {
	sr.statusMu.Lock()
	defer sr.statusMu.Unlock()
	for _, u := range update {
		u(sr.status)
	}
	logSyncer.Infof("syncing status: %s", sr.status.String())
}

// Status returns a copy of the current status.
func (sr *StatusReporter) Status() Status {
	return *sr.status
}

// NewStatusReporter initializes a new StatusReporter.
func NewStatusReporter() *StatusReporter {
	return &StatusReporter{
		status: newDefaultChainStatus(),
	}
}

// Status defines a structured used to represent the state of a chain store and syncer.
type Status struct {
	// The heaviest that has been fully validated.
	ValidatedHead types.TipSetKey
	// The height of ValidatedHead.
	ValidatedHeadHeight uint64
	// Unix time at which validation of chain at head SyncingTip began, zero if valdation hasn't started.
	ValidatedStarted int64

	// They key of the head of the chain currently being fetched/validator, or undef if none.
	SyncingHead types.TipSetKey
	// The height of SyncingHead.
	SyncingHeight uint64
	// Whether SyncingTip is trusted as a head far away from the validated head.
	SyncingTrusted bool
	// Whether SyncingHead has been validated.
	SyncingComplete bool
	// Whether SyncingHead has been fetched.
	SyncingFetchComplete bool

	// The key of the tipset currently being fetched
	FetchingHead types.TipSetKey
	// The height of FetchingHead
	FetchingHeight uint64
}

// NewDefaultChainStatus returns a ChainStaus with the default empty values.
func newDefaultChainStatus() *Status {
	return &Status{
		ValidatedHead:        types.UndefTipSet.Key(),
		ValidatedHeadHeight:  0,
		ValidatedStarted:     0,
		SyncingHead:          types.UndefTipSet.Key(),
		SyncingHeight:        0,
		SyncingTrusted:       false,
		SyncingComplete:      true,
		SyncingFetchComplete: true,
		FetchingHead:         types.UndefTipSet.Key(),
		FetchingHeight:       0,
	}
}

// String returns the Status as a string
func (s Status) String() string {
	return fmt.Sprintf("validatedHead=%s, validatedHeight=%d, validatedStarted=%d, syncingHead=%s, syncingHeight=%d, syncingTrusted=%t, syncingComplete=%t syncingFetchComplete=%t fetchingHead=%s, fetchingHeight=%d",
		s.ValidatedHead, s.ValidatedHeadHeight, s.ValidatedStarted,
		s.SyncingHead, s.SyncingHeight, s.SyncingTrusted, s.SyncingComplete, s.SyncingFetchComplete,
		s.FetchingHead, s.FetchingHeight)
}

// StatusUpdates defines a type for updateing syncer status.
type StatusUpdates func(*Status)

//
// Validation Updates
//
func validateHead(u types.TipSetKey) StatusUpdates {
	return func(s *Status) {
		s.ValidatedHead = u
	}
}

func validateHeight(u uint64) StatusUpdates {
	return func(s *Status) {
		s.ValidatedHeadHeight = u
	}
}

func validateStarted(u int64) StatusUpdates {
	return func(s *Status) {
		s.ValidatedStarted = u
	}
}

//
// Syncing Updates
//

func syncHead(u types.TipSetKey) StatusUpdates {
	return func(s *Status) {
		s.SyncingHead = u
	}
}

func syncHeight(u uint64) StatusUpdates {
	return func(s *Status) {
		s.SyncingHeight = u
	}
}

func syncTrusted(u bool) StatusUpdates {
	return func(s *Status) {
		s.SyncingTrusted = u
	}
}

func syncComplete(u bool) StatusUpdates {
	return func(s *Status) {
		s.SyncingComplete = u
	}
}

func syncFetchComplete(u bool) StatusUpdates {
	return func(s *Status) {
		s.SyncingFetchComplete = u
	}
}

//
// Fetching Updates
//

func fetchHead(u types.TipSetKey) StatusUpdates {
	return func(s *Status) {
		s.FetchingHead = u
	}
}
func fetchHeight(u uint64) StatusUpdates {
	return func(s *Status) {
		s.FetchingHeight = u
	}
}
