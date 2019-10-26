package chain

import (
	"fmt"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	logging "github.com/ipfs/go-log"
)

var logChainStatus = logging.Logger("chain/status")

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
	logChainStatus.Debugf("syncing status: %s", sr.status.String())
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

// Status defines a structure used to represent the state of a chain store and syncer.
type Status struct {
	// The heaviest TipSet that has been fully validated.
	ValidatedHead block.TipSetKey
	// The height of ValidatedHead.
	ValidatedHeadHeight uint64

	// They head of the chain currently being fetched/validated, or undef if none.
	SyncingHead block.TipSetKey
	// The height of SyncingHead.
	SyncingHeight uint64
	// Whether SyncingTip is trusted as a head far away from the validated head.
	SyncingTrusted bool
	// Unix time at which syncing of chain at SyncingHead began, zero if valdation hasn't started.
	SyncingStarted int64
	// Whether SyncingHead has been validated.
	SyncingComplete bool
	// Whether SyncingHead has been fetched.
	SyncingFetchComplete bool

	// The key of the tipset currently being fetched
	FetchingHead block.TipSetKey
	// The height of FetchingHead
	FetchingHeight uint64
}

// NewDefaultChainStatus returns a ChainStaus with the default empty values.
func newDefaultChainStatus() *Status {
	return &Status{
		ValidatedHead:        block.UndefTipSet.Key(),
		ValidatedHeadHeight:  0,
		SyncingHead:          block.UndefTipSet.Key(),
		SyncingHeight:        0,
		SyncingTrusted:       false,
		SyncingStarted:       0,
		SyncingComplete:      true,
		SyncingFetchComplete: true,
		FetchingHead:         block.UndefTipSet.Key(),
		FetchingHeight:       0,
	}
}

// String returns the Status as a string
func (s Status) String() string {
	return fmt.Sprintf("validatedHead=%s, validatedHeight=%d, syncingStarted=%d, syncingHead=%s, syncingHeight=%d, syncingTrusted=%t, syncingComplete=%t syncingFetchComplete=%t fetchingHead=%s, fetchingHeight=%d",
		s.ValidatedHead, s.ValidatedHeadHeight, s.SyncingStarted,
		s.SyncingHead, s.SyncingHeight, s.SyncingTrusted, s.SyncingComplete, s.SyncingFetchComplete,
		s.FetchingHead, s.FetchingHeight)
}

// StatusUpdates defines a type for ipdating syncer status.
type StatusUpdates func(*Status)

//
// Validation Updates
//
func validateHead(u block.TipSetKey) StatusUpdates {
	return func(s *Status) {
		s.ValidatedHead = u
	}
}

func validateHeight(u uint64) StatusUpdates {
	return func(s *Status) {
		s.ValidatedHeadHeight = u
	}
}

//
// Syncing Updates
//

func syncHead(u block.TipSetKey) StatusUpdates {
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

func syncingStarted(u int64) StatusUpdates {
	return func(s *Status) {
		s.SyncingStarted = u
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

func fetchHead(u block.TipSetKey) StatusUpdates {
	return func(s *Status) {
		s.FetchingHead = u
	}
}
func fetchHeight(u uint64) StatusUpdates {
	return func(s *Status) {
		s.FetchingHeight = u
	}
}
