package gas

import (
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/multiformats/go-multicodec"

	"github.com/filecoin-project/go-state-types/exitcode"

	"github.com/filecoin-project/venus/pkg/vm/runtime"
)

// GasTracker maintains the stateView of gas usage throughout the execution of a message.
type GasTracker struct { //nolint
	GasAvailable int64
	GasUsed      int64

	ExecutionTrace    types.ExecutionTrace
	NumActorsCreated  uint64    //nolint
	AllowInternal     bool      //nolint
	CallerValidated   bool      //nolint
	LastGasChargeTime time.Time //nolint
	LastGasCharge     *types.GasTrace
}

// NewGasTracker initializes a new empty gas tracker
func NewGasTracker(limit int64) *GasTracker {
	return &GasTracker{
		GasUsed:      0,
		GasAvailable: limit,
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

// EnableDetailedTracing has different behaviour in the LegacyVM and FVM.
// In the LegacyVM, it enables detailed gas tracing, slowing down execution.
// In the FVM, it enables execution traces, which are primarily used to observe subcalls.
var EnableDetailedTracing = os.Getenv("VENUS_VM_ENABLE_TRACING") == "1"

// TryCharge charges `amount` or `RemainingGas()`, whichever is smaller.
//
// Returns `True` if the there was enough gas To pay for `amount`.
func (t *GasTracker) TryCharge(gasCharge GasCharge) bool {
	toUse := gasCharge.Total()
	// code for https://github.com/filecoin-project/venus/issues/4610
	if EnableDetailedTracing {
		now := time.Now()
		if t.LastGasCharge != nil {
			t.LastGasCharge.TimeTaken = now.Sub(t.LastGasChargeTime)
		}

		gasTrace := types.GasTrace{
			Name: gasCharge.Name,

			TotalGas:   toUse,
			ComputeGas: gasCharge.ComputeGas,
			StorageGas: gasCharge.StorageGas,
		}

		t.ExecutionTrace.GasCharges = append(t.ExecutionTrace.GasCharges, &gasTrace)
		t.LastGasChargeTime = now
		t.LastGasCharge = &gasTrace
	}

	// overflow safe
	if t.GasUsed > t.GasAvailable-toUse {
		t.GasUsed = t.GasAvailable
		// return aerrors.Newf(exitcode.SysErrOutOfGas, "not enough gasCharge: used=%d, available=%d", t.GasUsed, t.GasAvailable)
		return false
	}
	t.GasUsed += toUse
	return true
}

func (t *GasTracker) FillMessageTrace(msg *types.Message) {
	paramsCodec := uint64(0)
	if len(msg.Params) > 0 {
		paramsCodec = uint64(multicodec.Cbor)
	}

	t.ExecutionTrace.Msg = types.MessageTrace{
		From:        msg.From,
		To:          msg.To,
		Value:       msg.Value,
		Method:      msg.Method,
		Params:      msg.Params,
		ParamsCodec: paramsCodec,
	}
}

func (t *GasTracker) FillReturnTrace(ret []byte, code exitcode.ExitCode) {
	retCodec := uint64(0)
	if len(ret) > 0 {
		retCodec = uint64(multicodec.Cbor)
	}
	t.ExecutionTrace.MsgRct = types.ReturnTrace{
		ExitCode:    code,
		Return:      ret,
		ReturnCodec: retCodec,
	}
}
