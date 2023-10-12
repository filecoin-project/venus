package vmcontext

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/venus/venus-shared/types"
)

const (
	// DefaultAvailableExecutionLanes is the number of available execution lanes; it is the bound of
	// concurrent active executions.
	// This is the default value in filecoin-ffi
	DefaultAvailableExecutionLanes = 4
	// DefaultPriorityExecutionLanes is the number of reserved execution lanes for priority computations.
	// This is purely userspace, but we believe it is a reasonable default, even with more available
	// lanes.
	DefaultPriorityExecutionLanes = 2
)

// the execution environment; see below for definition, methods, and initialization
var execution *executionEnv

// implementation of vm executor with simple sanity check preventing use after free.
type vmExecutor struct {
	vmi  Interface
	lane ExecutionLane
}

var _ Interface = (*vmExecutor)(nil)

func NewVMExecutor(vmi Interface, lane ExecutionLane) Interface {
	return &vmExecutor{vmi: vmi, lane: lane}
}

func (e *vmExecutor) ApplyMessage(ctx context.Context, cmsg types.ChainMsg) (*Ret, error) {
	token := execution.getToken(e.lane)
	defer token.Done()

	return e.vmi.ApplyMessage(ctx, cmsg)
}

func (e *vmExecutor) ApplyImplicitMessage(ctx context.Context, msg types.ChainMsg) (*Ret, error) {
	token := execution.getToken(e.lane)
	defer token.Done()

	return e.vmi.ApplyImplicitMessage(ctx, msg)
}

func (e *vmExecutor) Flush(ctx context.Context) (cid.Cid, error) {
	return e.vmi.Flush(ctx)
}

type executionToken struct {
	lane     ExecutionLane
	reserved int
}

func (token *executionToken) Done() {
	execution.putToken(token)
}

type executionEnv struct {
	mx   *sync.Mutex
	cond *sync.Cond

	// available executors
	available int
	// reserved executors
	reserved int
}

func (e *executionEnv) getToken(lane ExecutionLane) *executionToken {

	e.mx.Lock()
	defer e.mx.Unlock()

	switch lane {
	case ExecutionLaneDefault:
		for e.available <= e.reserved {
			e.cond.Wait()
		}

		e.available--

		return &executionToken{lane: lane, reserved: 0}

	case ExecutionLanePriority:
		for e.available == 0 {
			e.cond.Wait()
		}

		e.available--

		reserving := 0
		if e.reserved > 0 {
			e.reserved--
			reserving = 1
		}

		return &executionToken{lane: lane, reserved: reserving}

	default:
		// already checked at interface boundary in NewVM, so this is appropriate
		panic("bogus execution lane")
	}
}

func (e *executionEnv) putToken(token *executionToken) {
	e.mx.Lock()
	defer e.mx.Unlock()

	e.available++
	e.reserved += token.reserved

	// Note: Signal is unsound, because a priority token could wake up a non-priority
	// goroutnie and lead to deadlock. So Broadcast it must be.
	e.cond.Broadcast()

}

func init() {
	var err error

	available := DefaultAvailableExecutionLanes
	if concurrency := os.Getenv("LOTUS_FVM_CONCURRENCY"); concurrency != "" {
		available, err = strconv.Atoi(concurrency)
		if err != nil {
			panic(err)
		}
		if available > 24 {
			vmlog.Warnf("LOTUS_FVM_CONCURRENCY is set to a high value that can cause chain sync panics on network "+
				"migrations/upgrades, recommend 24 or less during network upgrades, current value: %v", available)
		}
	}

	priority := DefaultPriorityExecutionLanes
	if reserved := os.Getenv("LOTUS_FVM_CONCURRENCY_RESERVED"); reserved != "" {
		priority, err = strconv.Atoi(reserved)
		if err != nil {
			panic(err)
		}
	}

	// some sanity checks
	if available < 2 {
		panic("insufficient execution concurrency")
	}

	if available <= priority {
		panic("insufficient default execution concurrency")
	}

	mx := &sync.Mutex{}
	cond := sync.NewCond(mx)

	execution = &executionEnv{
		mx:        mx,
		cond:      cond,
		available: available,
		reserved:  priority,
	}
}
