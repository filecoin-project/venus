package vmcontext

import (
	"fmt"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/types"
	"time"

	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/gas"
)

// GasTracker maintains the stateView of gas usage throughout the execution of a message.
type GasTracker struct {
	gasAvailable int64
	gasUsed      int64

	executionTrace    types.ExecutionTrace
	numActorsCreated  uint64
	allowInternal     bool
	callerValidated   bool
	lastGasChargeTime time.Time
	lastGasCharge     *types.GasTrace
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker(limit gas.Unit) GasTracker {
	return GasTracker{
		gasUsed:      0,
		gasAvailable: int64(limit),
	}
}

// Charge will add the gas charge to the current method gas context.
//
// WARNING: this method will panic if there is no sufficient gas left.
func (t *GasTracker) Charge(gas gas.GasCharge, msg string, args ...interface{}) {
	if ok := t.TryCharge(gas); !ok {
		fmsg := fmt.Sprintf(msg, args...)
		runtime.Abortf(exitcode.SysErrOutOfGas, "gas limit %d exceeded with charge of %d: %s", t.gasAvailable, gas.Total(), fmsg)
	}
}

// TryCharge charges `amount` or `RemainingGas()``, whichever is smaller.
//
// Returns `True` if the there was enough gas to pay for `amount`.
func (t *GasTracker) TryCharge(gas gas.GasCharge) bool {
	toUse := gas.Total()
	//var callers [10]uintptr
	//cout := gruntime.Callers(2+skip, callers[:])

	now := time.Now() //build.Clock.Now()   todo add by force check here
	if t.lastGasCharge != nil {
		t.lastGasCharge.TimeTaken = now.Sub(t.lastGasChargeTime)
	}

	gasTrace := types.GasTrace{
		Name:  gas.Name,
		Extra: gas.Extra,

		TotalGas:   toUse,
		ComputeGas: gas.ComputeGas,
		StorageGas: gas.StorageGas,

		TotalVirtualGas:   gas.VirtualCompute*gas.GasComputeMulti + gas.VirtualStorage*gas.GasStorageMulti,
		VirtualComputeGas: gas.VirtualCompute,
		VirtualStorageGas: gas.VirtualStorage,

		//Callers: callers[:cout],
	}
	t.executionTrace.GasCharges = append(t.executionTrace.GasCharges, &gasTrace)
	t.lastGasChargeTime = now
	t.lastGasCharge = &gasTrace

	// overflow safe
	if t.gasUsed > t.gasAvailable-toUse {
		t.gasUsed = t.gasAvailable
		//return aerrors.Newf(exitcode.SysErrOutOfGas, "not enough gas: used=%d, available=%d", t.gasUsed, t.gasAvailable)
		return false
	}
	t.gasUsed += toUse
	return true
}
