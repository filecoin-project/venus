package gas

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-state-types/exitcode"

	types2 "github.com/filecoin-project/venus/pkg/types"
	"github.com/filecoin-project/venus/pkg/vm/runtime"
)

// GasTracker maintains the stateView of gas usage throughout the execution of a message.
type GasTracker struct { //nolint
	GasAvailable int64
	GasUsed      int64

	ExecutionTrace    types2.ExecutionTrace
	NumActorsCreated  uint64    //nolint
	AllowInternal     bool      //nolint
	CallerValidated   bool      //nolint
	LastGasChargeTime time.Time //nolint
	LastGasCharge     *types2.GasTrace
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker(limit types2.Unit) *GasTracker {
	return &GasTracker{
		GasUsed:      0,
		GasAvailable: int64(limit),
	}
}

// Charge will add the gas charge To the current Method gas context.
//
// WARNING: this Method will panic if there is no sufficient gas left.
func (t *GasTracker) Charge(gas GasCharge, msg string, args ...interface{}) {
	if ok := t.TryCharge(gas); !ok {
		fmsg := fmt.Sprintf(msg, args...)
		runtime.Abortf(exitcode.SysErrOutOfGas, "gas limit %d exceeded with charge of %d: %s", t.GasAvailable, gas.Total(), fmsg)
	}
}

// TryCharge charges `amount` or `RemainingGas()``, whichever is smaller.
//
// Returns `True` if the there was enough gas To pay for `amount`.
func (t *GasTracker) TryCharge(gasCharge GasCharge) bool {
	toUse := gasCharge.Total()
	var callers [10]uintptr
	cout := 0 //gruntime.Callers(2+skip, callers[:])

	now := time.Now()
	if t.LastGasCharge != nil {
		t.LastGasCharge.TimeTaken = now.Sub(t.LastGasChargeTime)
	}

	gasTrace := types2.GasTrace{
		Name:  gasCharge.Name,
		Extra: gasCharge.Extra,

		TotalGas:   toUse,
		ComputeGas: gasCharge.ComputeGas,
		StorageGas: gasCharge.StorageGas,

		//TotalVirtualGas:   gasCharge.VirtualCompute*GasComputeMulti + gasCharge.VirtualStorage*GasStorageMulti,
		TotalVirtualGas:   gasCharge.VirtualCompute + gasCharge.VirtualStorage,
		VirtualComputeGas: gasCharge.VirtualCompute,
		VirtualStorageGas: gasCharge.VirtualStorage,

		Callers: callers[:cout],
	}

	if gasTrace.VirtualStorageGas == 0 {
		gasTrace.VirtualStorageGas = gasTrace.StorageGas
	}
	if gasTrace.VirtualComputeGas == 0 {
		gasTrace.VirtualComputeGas = gasTrace.ComputeGas
	}

	t.ExecutionTrace.GasCharges = append(t.ExecutionTrace.GasCharges, &gasTrace)
	t.LastGasChargeTime = now
	t.LastGasCharge = &gasTrace

	// overflow safe
	if t.GasUsed > t.GasAvailable-toUse {
		t.GasUsed = t.GasAvailable
		//return aerrors.Newf(exitcode.SysErrOutOfGas, "not enough gasCharge: used=%d, available=%d", t.GasUsed, t.GasAvailable)
		return false
	}
	t.GasUsed += toUse
	return true
}
