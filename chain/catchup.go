package chain

import (
	"context"
	"time"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/clock"
	"github.com/filecoin-project/go-filecoin/types"
)

var logCatchup = logging.Logger("chain.catchup")

type catchupStore interface {
	GetHead() types.TipSetKey
	GetTipSet(tsKey types.TipSetKey) (types.TipSet, error)
}

type catchupTracker interface {
	SelectHead() (*types.ChainInfo, error)
	UpdateTrusted(context.Context) error
}

type catchupSyncer interface {
	HandleNewTipSet(context.Context, *types.ChainInfo, bool) error
	Status() Status
}

type catchupComplete func(newHead *types.ChainInfo, curHead types.TipSet) (bool, error)

// CatchupSyncer is used to catch a chain up to its trusted peers.
type CatchupSyncer struct {
	tracker  catchupTracker
	syncer   catchupSyncer
	store    catchupStore
	done     catchupComplete
	cancelFn context.CancelFunc
}

// NewCatchupSyncer returns a new CatchupSyncer.
func NewCatchupSyncer(t catchupTracker, sy catchupSyncer, st catchupStore, done catchupComplete) *CatchupSyncer {
	return &CatchupSyncer{
		tracker: t,
		syncer:  sy,
		store:   st,
		done:    done,
	}
}

// Start starts a go routine to performe the catchup process. The catch up routine will run each time
// `tick` ticks.
func (cu *CatchupSyncer) Start(ctx context.Context, tick clock.Ticker) (<-chan struct{}, <-chan error) {
	logCatchup.Info("starting chain catchup syncer")

	// setup a context to allow the catchupper to be cancled
	cctx, cancel := context.WithCancel(ctx)
	cu.cancelFn = cancel

	// setup channels to report status
	errors := make(chan error)
	done := make(chan struct{})
	go func() {
		// when this exits clean up and signal that we are done.
		defer func() {
			done <- struct{}{}
			close(errors)
			close(done)
			logCatchup.Info("chain catchup complete, catchup syncer shutting down")
			cu.cancelFn()
		}()
		for {
			select {
			case <-tick.Chan():
				success, retry, err := cu.CatchUp(cctx)
				if success {
					logCatchup.Infof("completed chain catchup status: %s", cu.syncer.Status().String())
					return
				}
				if retry {
					logCatchup.Infof("chain catchup encountered retryable error: %s", err.Error())
					continue
				}
				if err != nil {
					errors <- err
					logCatchup.Errorf("chain catchup encountered unretryable error: %s", err.Error())
					return
				}
				logCatchup.Infof("chain catchup status: %s", cu.syncer.Status().String())
			case <-cctx.Done():
				return
			}
		}
	}()
	return done, errors
}

// Stop stops the go routine performing the catchup process.
func (cu *CatchupSyncer) Stop() {
	cu.cancelFn()
}

var catchupUpdatePeersTo = time.Second * 10

// CatchUp passes the best head returned from the PeerTracker and the nodes current head to the done
// function, if the done function returns true the operation is successful.
func (cu *CatchupSyncer) CatchUp(ctx context.Context) (success bool, retry bool, err error) {
	// It bothers me that tick could have a shorter duration than the timeout...
	updtCtx, cancel := context.WithTimeout(ctx, catchupUpdatePeersTo)
	defer cancel()

	if err = cu.tracker.UpdateTrusted(updtCtx); err != nil {
		retry = true
		success = false
		return
	}

	newHead, err := cu.tracker.SelectHead()
	if err != nil {
		retry = true
		success = false
		return
	}

	// all errors after this point are non-retryable
	retry = false

	curHead, err := cu.store.GetTipSet(cu.store.GetHead())
	if err != nil {
		success = false
		return
	}

	if err = cu.syncer.HandleNewTipSet(ctx, newHead, true); err != nil {
		success = false
		return
	}

	success, err = cu.done(newHead, curHead)
	return
}
