package vmcontext

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-state-types/exitcode"

	types2 "github.com/filecoin-project/venus/internal/pkg/types"
	"github.com/filecoin-project/venus/internal/pkg/vm/gas"
	"github.com/filecoin-project/venus/internal/pkg/vm/internal/runtime"
)

// GasTracker maintains the stateView of gas usage throughout the execution of a message.
type GasTracker struct {
	gasAvailable int64
	gasUsed      int64

	executionTrace    types2.ExecutionTrace
	numActorsCreated  uint64    //nolint
	allowInternal     bool      //nolint
	callerValidated   bool      //nolint
	lastGasChargeTime time.Time //nolint
	lastGasCharge     *types2.GasTrace
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker(limit types2.Unit) *GasTracker {
	return &GasTracker{
		gasUsed:      0,
		gasAvailable: int64(limit),
	}
}

// Charge will add the gas charge To the current Method gas context.
//
// WARNING: this Method will panic if there is no sufficient gas left.
func (t *GasTracker) Charge(gas gas.GasCharge, msg string, args ...interface{}) {
	if ok := t.TryCharge(gas); !ok {
		fmsg := fmt.Sprintf(msg, args...)
		runtime.Abortf(exitcode.SysErrOutOfGas, "gas limit %d exceeded with charge of %d: %s", t.gasAvailable, gas.Total(), fmsg)
	}
}

// TryCharge charges `amount` or `RemainingGas()``, whichever is smaller.
//
// Returns `True` if the there was enough gas To pay for `amount`.
func (t *GasTracker) TryCharge(gasCharge gas.GasCharge) bool {
	toUse := gasCharge.Total()
	//var callers [10]uintptr
	//cout := gruntime.Callers(2+skip, callers[:])

	now := time.Now()
	if t.lastGasCharge != nil {
		t.lastGasCharge.TimeTaken = now.Sub(t.lastGasChargeTime)
	}

	gasTrace := types2.GasTrace{
		Name:  gasCharge.Name,
		Extra: gasCharge.Extra,

		TotalGas:   toUse,
		ComputeGas: gasCharge.ComputeGas,
		StorageGas: gasCharge.StorageGas,

		TotalVirtualGas:   gasCharge.VirtualCompute*gas.GasComputeMulti + gasCharge.VirtualStorage*gas.GasStorageMulti,
		VirtualComputeGas: gasCharge.VirtualCompute,
		VirtualStorageGas: gasCharge.VirtualStorage,

		//Callers: callers[:cout],
	}

	t.executionTrace.GasCharges = append(t.executionTrace.GasCharges, &gasTrace)
	t.lastGasChargeTime = now
	t.lastGasCharge = &gasTrace

	// overflow safe
	if t.gasUsed > t.gasAvailable-toUse {
		t.gasUsed = t.gasAvailable
		//return aerrors.Newf(exitcode.SysErrOutOfGas, "not enough gasCharge: used=%d, available=%d", t.gasUsed, t.gasAvailable)
		return false
	}
	t.gasUsed += toUse
	return true
}
