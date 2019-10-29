package status

import (
	"fmt"
	"sync"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	logging "github.com/ipfs/go-log"
)

// Reporter defines an interface to updating and reporting the status of the blockchain.
type Reporter interface {
	UpdateStatus(...UpdateFn)
	Status() Status
}

// Status defines a structure used to represent the state of a chain store and syncer.
type Status struct {
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

type reporter struct {
	statusMu sync.Mutex
	status   *Status
}

// UpdateFn defines a type for ipdating syncer status.
type UpdateFn func(*Status)

var logChainStatus = logging.Logger("status")

// NewReporter initializes a new status reporter.
func NewReporter() Reporter {
	return &reporter{
		status: NewDefaultChainStatus(),
	}
}

// NewDefaultChainStatus returns a ChainStaus with the default empty values.
func NewDefaultChainStatus() *Status {
	return &Status{
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
	return fmt.Sprintf("syncingStarted=%d, syncingHead=%s, syncingHeight=%d, syncingTrusted=%t, syncingComplete=%t syncingFetchComplete=%t fetchingHead=%s, fetchingHeight=%d",
		s.SyncingStarted,
		s.SyncingHead, s.SyncingHeight, s.SyncingTrusted, s.SyncingComplete, s.SyncingFetchComplete,
		s.FetchingHead, s.FetchingHeight)
}

// UpdateStatus updates the status heald by StatusReporter.
func (sr *reporter) UpdateStatus(update ...UpdateFn) {
	sr.statusMu.Lock()
	defer sr.statusMu.Unlock()
	for _, u := range update {
		u(sr.status)
	}
	logChainStatus.Debugf("syncing status: %s", sr.status.String())
}

// Status returns a copy of the current status.
func (sr *reporter) Status() Status {
	return *sr.status
}

//
// Syncing Updates
//

// SyncHead updates the head.
func SyncHead(u block.TipSetKey) UpdateFn {
	return func(s *Status) {
		s.SyncingHead = u
	}
}

// SyncHeight updates the head.
func SyncHeight(u uint64) UpdateFn {
	return func(s *Status) {
		s.SyncingHeight = u
	}
}

// SyncTrusted updates the trusted.
func SyncTrusted(u bool) UpdateFn {
	return func(s *Status) {
		s.SyncingTrusted = u
	}
}

// SyncingStarted marks the syncing as started.
func SyncingStarted(u int64) UpdateFn {
	return func(s *Status) {
		s.SyncingStarted = u
	}
}

// SyncComplete marks the fetch as complete.
func SyncComplete(u bool) UpdateFn {
	return func(s *Status) {
		s.SyncingComplete = u
	}
}

// SyncFetchComplete determines if the fetch is complete.
func SyncFetchComplete(u bool) UpdateFn {
	return func(s *Status) {
		s.SyncingFetchComplete = u
	}
}

//
// Fetching Updates
//

// FetchHead gets the the head.
func FetchHead(u block.TipSetKey) UpdateFn {
	return func(s *Status) {
		s.FetchingHead = u
	}
}

// FetchHeight gets the height.
func FetchHeight(u uint64) UpdateFn {
	return func(s *Status) {
		s.FetchingHeight = u
	}
}
