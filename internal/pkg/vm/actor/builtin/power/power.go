package power

import (
	"context"
	"math/big"
	"reflect"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
)

func init() {
	encoding.RegisterIpldCborType(State{})
	encoding.RegisterIpldCborType(TableEntry{})
}

// Actor provides bookkeeping for the storage power of registered mienrs.
// It updates power based on faults and storage proofs.
// It also tracks pledge collateral conditions.
type Actor struct{}

// State keeps track of power and collateral of registered miner actors
type State struct {
	// PowerTable is a lookup mapping actorAddr -> PowerTableEntry
	PowerTable cid.Cid `refmt:",omitempty"`
}

// TableEntry tracks a single miner actor's power and collateral
type TableEntry struct {
	ActivePower            *types.BytesAmount
	InactivePower          *types.BytesAmount
	AvailableBalance       types.AttoFIL
	LockedPledgeCollateral types.AttoFIL
}

// Actor Methods
const (
	CreateStorageMiner types.MethodID = iota + 32
	RemoveStorageMiner
	GetTotalPower
	EnsurePledgeCollateralSatisfied
	ProcessPowerReport
	GetPowerReport
	ProcessFaultReport
	// ReportConsensusFault
	// Surprise
	// AddBalance ? (review: is this a runtime builtin?)
	// WithdrawBalance ? (review: is this a runtime builtin?)
)

// NewActor returns a new power actor
func NewActor() *actor.Actor {
	return actor.NewActor(types.PowerActorCodeCid, types.ZeroAttoFIL)
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	CreateStorageMiner: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address, abi.Address, abi.PeerID, abi.BytesAmount},
		Return: []abi.Type{abi.Address},
	},
	RemoveStorageMiner: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: nil,
	},
	GetTotalPower: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.BytesAmount},
	},
	ProcessPowerReport: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.PowerReport, abi.Address},
		Return: nil,
	},
	GetPowerReport: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.Address},
		Return: []abi.Type{abi.PowerReport},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case CreateStorageMiner:
		return reflect.ValueOf((*impl)(a).createStorageMiner), signatures[CreateStorageMiner], true
	case RemoveStorageMiner:
		return reflect.ValueOf((*impl)(a).removeStorageMiner), signatures[RemoveStorageMiner], true
	case GetTotalPower:
		return reflect.ValueOf((*impl)(a).getTotalPower), signatures[GetTotalPower], true
	case ProcessPowerReport:
		return reflect.ValueOf((*impl)(a).processPowerReport), signatures[ProcessPowerReport], true
	case GetPowerReport:
		return reflect.ValueOf((*impl)(a).getPowerReport), signatures[GetPowerReport], true
	default:
		return nil, nil, false
	}
}

// InitializeState stores the actor's initial data structure.
func (*Actor) InitializeState(storage runtime.Storage, _ interface{}) error {
	initStorage := &State{}
	stateBytes, err := encoding.Encode(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, cid.Undef)
}

//
// vm methods for actor
//

type impl Actor

const (
	// ErrDeleteMinerWithPower signals that RemoveStorageMiner was called on an actor with nonzero power
	ErrDeleteMinerWithPower = 100
	// ErrUnknownEntry entry is returned when the actor attempts to access a power table entry at an address not in the power table
	ErrUnknownEntry = 101
	// ErrDuplicateEntry is returned when there is an attempt to create a new power table entry at an existing addrErr
	ErrDuplicateEntry = 102
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrDeleteMinerWithPower: errors.NewCodedRevertError(ErrDeleteMinerWithPower, "cannot delete miner with power from power table"),
	ErrUnknownEntry:         errors.NewCodedRevertError(ErrUnknownEntry, "cannot find address in power table"),
	ErrDuplicateEntry:       errors.NewCodedRevertError(ErrDuplicateEntry, "duplicate create power table entry attempt"),
}

// Dragons: shouldnt this be calling the initacor to create an actor?
type invocationContext interface {
	runtime.InvocationContext
	LegacyAddressForNewActor() (address.Address, error)
}

// CreateStorageMiner creates a new record of a miner in the power table.
func (*impl) createStorageMiner(vmctx invocationContext, ownerAddr, workerAddr address.Address, pid peer.ID, sectorSize *types.BytesAmount) (address.Address, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		actorCodeCid := types.MinerActorCodeCid
		epoch := vmctx.Runtime().CurrentEpoch()
		if epoch.Equal(types.NewBlockHeight(0)) {
			actorCodeCid = types.BootstrapMinerActorCodeCid
		}

		initParams := []interface{}{vmctx.Message().Caller(), vmctx.Message().Caller(), pid, sectorSize}

		// create miner actor by messaging the init actor and sending it collateral
		ret, _, err := vmctx.Runtime().Send(address.InitAddress, initactor.Exec, vmctx.Message().ValueReceived(), []interface{}{actorCodeCid, initParams})
		if err != nil {
			return nil, err
		}

		addr, err := address.NewFromBytes(ret[0])
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not convert init.Exec return value to address")
		}

		// retrieve id to key miner
		actorIDAddr, err := retreiveActorID(vmctx.Runtime(), addr)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not retrieve actor id addrs after initializing actor")
		}

		// Update power table.
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable, func(lookup storage.Lookup) error {
			// Do not overwrite table entry if it already exists
			err := lookup.Find(ctx, actorIDAddr.String(), nil)
			if err != hamt.ErrNotFound { // we expect to not find the power table entry
				if err == nil {
					return Errors[ErrDuplicateEntry]
				}
				return errors.FaultErrorWrapf(err, "Error looking for new entry in power table at addres %s", actorIDAddr.String())
			}

			// Create fresh entry
			err = lookup.Set(ctx, actorIDAddr.String(), TableEntry{
				ActivePower:            types.NewBytesAmount(0),
				InactivePower:          types.NewBytesAmount(0),
				AvailableBalance:       types.ZeroAttoFIL,
				LockedPledgeCollateral: types.ZeroAttoFIL,
			})
			if err != nil {
				return errors.FaultErrorWrapf(err, "Could not set power table at address: %s", actorIDAddr.String())
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = newPowerTable
		return addr, nil
	})
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	return ret.(address.Address), 0, nil
}

// retriveActorId uses init actor to map an actorAddress to an id address
func retreiveActorID(vmctx runtime.Runtime, actorAddr address.Address) (address.Address, error) {
	ret, _, err := vmctx.Send(address.InitAddress, initactor.GetActorIDForAddress, types.ZeroAttoFIL, []interface{}{actorAddr})
	if err != nil {
		return address.Undef, err
	}

	actorIDVal, err := abi.Deserialize(ret[0], abi.Integer)
	if err != nil {
		return address.Undef, errors.FaultErrorWrap(err, "could not convert actor id to big.Int")
	}

	return address.NewIDAddress(actorIDVal.Val.(*big.Int).Uint64())
}

// RemoveStorageMiner removes the given miner address from the power table.  This call will fail if
// the miner has any power remaining in the table or if the actor does not already exit in the table.
func (*impl) removeStorageMiner(vmctx invocationContext, delAddr address.Address) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	// TODO #3649 we need proper authentication.  Totally insecure as it is.

	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable, func(lookup storage.Lookup) error {
			// Find entry to delete.
			var delEntry TableEntry
			err := lookup.Find(ctx, delAddr.String(), &delEntry)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return errors.FaultErrorWrapf(err, "Could not retrieve power table entry with ID: %s", delAddr.String())
			}

			// Never delete an entry that still has power
			if delEntry.ActivePower.IsPositive() || delEntry.InactivePower.IsPositive() {
				return Errors[ErrDeleteMinerWithPower]
			}

			// All clear to delete
			return lookup.Delete(ctx, delAddr.String())
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = newPowerTable
		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}
	return 0, nil
}

// GetTotalPower returns the total power (in bytes) held by all miners registered in the system
func (*impl) getTotalPower(vmctx invocationContext) (*types.BytesAmount, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	// TODO #3649 we need proper authentication. Totally insecure without.

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()
		total := types.NewBytesAmount(0)
		err := actor.WithLookupForReading(ctx, vmctx.Runtime().Storage(), state.PowerTable, func(lookup storage.Lookup) error {
			// TODO https://github.com/filecoin-project/specs/issues/634 this is inefficient
			return lookup.ForEachValue(ctx, TableEntry{}, func(k string, value interface{}) error {
				entry, ok := value.(TableEntry)
				if !ok {
					return errors.NewFaultError("Expected TableEntry from power table lookup")
				}
				total = total.Add(entry.ActivePower)
				total = total.Add(entry.InactivePower)
				return nil
			})
		})
		return total, err
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}
	return ret.(*types.BytesAmount), 0, nil
}

func (*impl) getPowerReport(vmctx invocationContext, addr address.Address) (types.PowerReport, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return types.PowerReport{}, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()
		var report types.PowerReport
		err := actor.WithLookupForReading(ctx, vmctx.Runtime().Storage(), state.PowerTable, func(lookup storage.Lookup) error {
			err := lookup.Find(ctx, addr.String(), &report)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return errors.FaultErrorWrapf(err, "Could not retrieve power table entry with ID: %s", addr.String())
			}
			return nil
		})
		return report, err
	})
	if err != nil {
		return types.PowerReport{}, errors.CodeError(err), err
	}
	return ret.(types.PowerReport), 0, nil
}

// ProcessPowerReport updates a registered miner's power table entry according to the power report.
func (*impl) processPowerReport(vmctx invocationContext, report types.PowerReport, updateAddr address.Address) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable, func(lookup storage.Lookup) error {
			// Find entry to update.
			var updateEntry TableEntry
			err := lookup.Find(ctx, updateAddr.String(), &updateEntry)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return errors.FaultErrorWrapf(err, "Could not retrieve power table entry with ID: %s", updateAddr.String())
			}
			// All clear to update
			return lookup.Set(ctx, updateAddr.String(), report)
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = newPowerTable
		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}
	return 0, nil
}
