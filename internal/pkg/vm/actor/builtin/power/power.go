package power

import (
	"context"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/pattern"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/storage"
	"github.com/filecoin-project/specs-actors/actors/builtin"
)

func init() {
	encoding.RegisterIpldCborType(State{})
	encoding.RegisterIpldCborType(TableEntry{})
}

// Actor provides bookkeeping for the storage power of registered miners.
// It updates power based on faults and storage proofs.
// It also tracks pledge collateral conditions.
type Actor struct{}

// State keeps track of power and collateral of registered miner actors
type State struct {
	// PowerTable is a lookup mapping actorAddr -> PowerTableEntry
	PowerTable enccid.Cid
}

// TableEntry tracks a single miner actor's power and collateral
type TableEntry struct {
	ActivePower            *types.BytesAmount
	InactivePower          *types.BytesAmount
	AvailableBalance       types.AttoFIL
	LockedPledgeCollateral types.AttoFIL
	SectorSize             *types.BytesAmount
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
	GetSectorSize
	// ReportConsensusFault
	// Surprise
	// AddBalance ? (review: is this a runtime builtin?)
	// WithdrawBalance ? (review: is this a runtime builtin?)
)

// NewActor returns a new power actor
func NewActor() *actor.Actor {
	return actor.NewActor(builtin.StoragePowerActorCodeID, abi.NewTokenAmount(0))
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.Actor = (*Actor)(nil)

// Exports implements `dispatch.Actor`
func (a *Actor) Exports() []interface{} {
	return []interface{}{
		CreateStorageMiner: (*impl)(a).createStorageMiner,
		ProcessPowerReport: (*impl)(a).processPowerReport,
	}
}

// InitializeState stores the actor's initial data structure.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, _ interface{}) error {
	handle.Create(&State{})
	return nil
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
	ErrDeleteMinerWithPower: fmt.Errorf("cannot delete miner with power from power table"),
	ErrUnknownEntry:         fmt.Errorf("cannot find address in power table"),
	ErrDuplicateEntry:       fmt.Errorf("duplicate create power table entry attempt"),
}

// CreateStorageMinerParams is the params for the CreateStorageMiner method.
type CreateStorageMinerParams struct {
	OwnerAddr  address.Address
	WorkerAddr address.Address
	PeerID     peer.ID
	SectorSize *types.BytesAmount
}

// CreateStorageMiner creates a new record of a miner in the power table.
func (*impl) createStorageMiner(vmctx runtime.InvocationContext, params CreateStorageMinerParams) address.Address {
	vmctx.ValidateCaller(pattern.Any{})

	initParams := miner.ConstructorParams{
		OwnerAddr:  vmctx.Message().Caller(),
		WorkerAddr: vmctx.Message().Caller(),
		PeerID:     params.PeerID,
		SectorSize: params.SectorSize,
	}

	constructorParams, err := encoding.Encode(initParams)
	if err != nil {
		panic(err)
	}

	actorCodeCid := builtin.StorageMinerActorCodeID
	epoch := vmctx.Runtime().CurrentEpoch()
	if epoch == 0 {
		actorCodeCid = types.BootstrapMinerActorCodeCid
	}

	// create miner actor by messaging the init actor and sending it collateral
	ret := vmctx.Send(vmaddr.InitAddress, initactor.ExecMethodID, vmctx.Message().ValueReceived(), initactor.ExecParams{
		ActorCodeCid:      actorCodeCid,
		ConstructorParams: constructorParams,
	})

	actorIDAddr := ret.(address.Address)

	var state State
	ret, err = vmctx.State().Transaction(&state, func() (interface{}, error) {
		// Update power table.
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			// Do not overwrite table entry if it already exists
			err := lookup.Find(ctx, string(actorIDAddr.Bytes()), nil)
			if err != hamt.ErrNotFound { // we expect to not find the power table entry
				if err == nil {
					return Errors[ErrDuplicateEntry]
				}
				return fmt.Errorf("Error looking for new entry in power table at addres %s", actorIDAddr)
			}

			// Create fresh entry
			err = lookup.Set(ctx, string(actorIDAddr.Bytes()), TableEntry{
				ActivePower:            types.NewBytesAmount(0),
				InactivePower:          types.NewBytesAmount(0),
				AvailableBalance:       types.ZeroAttoFIL,
				LockedPledgeCollateral: types.ZeroAttoFIL,
				SectorSize:             params.SectorSize,
			})
			if err != nil {
				fmt.Printf("here it is: %s\n", err)
				return fmt.Errorf("Could not set power table at address: %s", actorIDAddr)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = enccid.NewCid(newPowerTable)
		return actorIDAddr, nil
	})
	if err != nil {
		fmt.Printf("power actor panic %s\n", err)
		panic(err)
	}

	return ret.(address.Address)
}

// RemoveStorageMiner removes the given miner address from the power table.  This call will fail if
// the miner has any power remaining in the table or if the actor does not already exit in the table.
func (*impl) removeStorageMiner(vmctx runtime.InvocationContext, delAddr address.Address) (uint8, error) {
	// TODO #3649 we need proper authentication.  Totally insecure as it is.

	var state State
	_, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			// Find entry to delete.
			var delEntry TableEntry
			err := lookup.Find(ctx, string(delAddr.Bytes()), &delEntry)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return fmt.Errorf("Could not retrieve power table entry with ID: %s", delAddr)
			}

			// Never delete an entry that still has power
			if delEntry.ActivePower.IsPositive() || delEntry.InactivePower.IsPositive() {
				return Errors[ErrDeleteMinerWithPower]
			}

			// All clear to delete
			return lookup.Delete(ctx, string(delAddr.Bytes()))
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = enccid.NewCid(newPowerTable)
		return nil, nil
	})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

// GetTotalPower returns the total power (in bytes) held by all miners registered in the system
func (*impl) getTotalPower(vmctx runtime.InvocationContext) (*types.BytesAmount, uint8, error) {
	// TODO #3649 we need proper authentication. Totally insecure without.

	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		ctx := context.Background()
		total := types.NewBytesAmount(0)
		err := actor.WithLookupForReading(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			// TODO https://github.com/filecoin-project/specs/issues/634 this is inefficient
			return lookup.ForEachValue(ctx, TableEntry{}, func(k string, value interface{}) error {
				entry, ok := value.(TableEntry)
				if !ok {
					return fmt.Errorf("Expected TableEntry from power table lookup")
				}
				total = total.Add(entry.ActivePower)
				total = total.Add(entry.InactivePower)
				return nil
			})
		})
		return total, err
	})
	if err != nil {
		return nil, 1, err
	}
	return ret.(*types.BytesAmount), 0, nil
}

func (*impl) getPowerReport(vmctx runtime.InvocationContext, addr address.Address) (types.PowerReport, uint8, error) {
	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		ctx := context.Background()
		var tableEntry TableEntry
		var report types.PowerReport
		err := actor.WithLookupForReading(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			err := lookup.Find(ctx, string(addr.Bytes()), &tableEntry)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return fmt.Errorf("Could not retrieve power table entry with ID: %s", addr)
			}
			report.ActivePower = tableEntry.ActivePower
			report.InactivePower = tableEntry.InactivePower
			return nil
		})
		return report, err
	})
	if err != nil {
		return types.PowerReport{}, 1, err
	}
	return ret.(types.PowerReport), 0, nil
}

func (*impl) getSectorSize(vmctx runtime.InvocationContext, addr address.Address) (*types.BytesAmount, uint8, error) {
	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		ctx := context.Background()
		ss := types.NewBytesAmount(0)
		err := actor.WithLookupForReading(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			return lookup.ForEachValue(ctx, TableEntry{}, func(k string, value interface{}) error {
				entry, ok := value.(TableEntry)
				if !ok {
					return fmt.Errorf("Expected TableEntry from power table lookup")
				}
				ss = entry.SectorSize
				return nil
			})
		})
		return ss, err
	})
	if err != nil {
		return nil, 1, err
	}
	return ret.(*types.BytesAmount), 0, nil
}

// ProcessPowerReportParams is what is says.
type ProcessPowerReportParams struct {
	Report     types.PowerReport
	UpdateAddr address.Address
}

// ProcessPowerReport updates a registered miner's power table entry according to the power report.
func (*impl) processPowerReport(vmctx runtime.InvocationContext, params ProcessPowerReportParams) {
	vmctx.ValidateCaller(pattern.Any{})

	var state State
	_, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		ctx := context.Background()
		newPowerTable, err := actor.WithLookup(ctx, vmctx.Runtime().Storage(), state.PowerTable.Cid, func(lookup storage.Lookup) error {
			// Find entry to update.
			var updateEntry TableEntry
			err := lookup.Find(ctx, string(params.UpdateAddr.Bytes()), &updateEntry)
			if err != nil {
				if err == hamt.ErrNotFound {
					return Errors[ErrUnknownEntry]
				}
				return fmt.Errorf("Could not retrieve power table entry with ID: %s", params.UpdateAddr)
			}
			// All clear to update
			updateEntry.ActivePower = params.Report.ActivePower
			updateEntry.InactivePower = params.Report.InactivePower
			return lookup.Set(ctx, string(params.UpdateAddr.Bytes()), updateEntry)
		})
		if err != nil {
			return nil, err
		}
		state.PowerTable = enccid.NewCid(newPowerTable)
		return nil, nil
	})
	if err != nil {
		panic(err)
	}
}
