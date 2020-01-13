package storagemarket

import (
	"context"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/abi"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	internal "github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/errors"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

func init() {
	encoding.RegisterIpldCborType(struct{}{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, and keeping track of the total storage power in the network.
type Actor struct{}

// State is the storage market's storage.
type State struct {
	Miners cid.Cid `refmt:",omitempty"`

	// TODO: Determine correct unit of measure. Could be denominated in the
	// smallest sector size supported by the network.
	//
	// See: https://github.com/filecoin-project/specs/issues/6
	//
	TotalCommittedStorage *types.BytesAmount

	ProofsMode types.ProofsMode
}

// Actor methods
const (
	CreateStorageMiner types.MethodID = iota + 32
	UpdateStorage
	GetTotalStorage
	GetProofsMode
	GetLateMiners
	PublishStorageDeals
	PreCommitSector
	CommitSector
)

// NewActor returns a new storage market actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.StorageMarketActorCodeCid, types.ZeroAttoFIL)
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.ExecutableActor = (*Actor)(nil)

var signatures = dispatch.Exports{
	CreateStorageMiner: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.BytesAmount, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	UpdateStorage: &dispatch.FunctionSignature{
		Params: []abi.Type{abi.BytesAmount},
		Return: nil,
	},
	GetTotalStorage: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.BytesAmount},
	},
	GetProofsMode: &dispatch.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.ProofsMode},
	},
	GetLateMiners: &dispatch.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.MinerPoStStates},
	},
}

// Method returns method definition for a given method id.
func (a *Actor) Method(id types.MethodID) (dispatch.Method, *dispatch.FunctionSignature, bool) {
	switch id {
	case CreateStorageMiner:
		return reflect.ValueOf((*impl)(a).createStorageMiner), signatures[CreateStorageMiner], true
	case UpdateStorage:
		return reflect.ValueOf((*impl)(a).updateStorage), signatures[UpdateStorage], true
	case GetTotalStorage:
		return reflect.ValueOf((*impl)(a).getTotalStorage), signatures[GetTotalStorage], true
	case GetProofsMode:
		return reflect.ValueOf((*impl)(a).getProofsMode), signatures[GetProofsMode], true
	case GetLateMiners:
		return reflect.ValueOf((*impl)(a).getLateMiners), signatures[GetLateMiners], true
	default:
		return nil, nil, false
	}
}

// InitializeState stores the actor's initial data structure.
func (*Actor) InitializeState(storage runtime.LegacyStorage, proofsModeInterface interface{}) error {
	proofsMode, ok := proofsModeInterface.(types.ProofsMode)
	if !ok {
		return errors.NewRevertError("storage market actor init parameter is not a proofs mode")
	}

	initStorage := &State{
		TotalCommittedStorage: types.NewBytesAmount(0),
		ProofsMode:            proofsMode,
	}
	stateBytes, err := encoding.Encode(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.LegacyCommit(id, cid.Undef)
}

//
// vm methods for actor
//

type impl Actor

const (
	// ErrUnknownMiner indicates a pledge under the MinimumPledge.
	ErrUnknownMiner = 34
	// ErrUnsupportedSectorSize indicates that the sector size is incompatible with the proofs mode.
	ErrUnsupportedSectorSize = 44
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrUnknownMiner:          errors.NewCodedRevertErrorf(ErrUnknownMiner, "unknown miner"),
	ErrUnsupportedSectorSize: errors.NewCodedRevertErrorf(ErrUnsupportedSectorSize, "sector size is not supported"),
}

// CreateStorageMiner creates a new miner which will commit sectors of the
// given size. The miners collateral is set by the value in the message.
func (*impl) createStorageMiner(vmctx runtime.InvocationContext, sectorSize *types.BytesAmount, pid peer.ID) (address.Address, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		if !isSupportedSectorSize(state.ProofsMode, sectorSize) {
			return nil, Errors[ErrUnsupportedSectorSize]
		}

		actorCodeCid := types.MinerActorCodeCid
		epoch := vmctx.Runtime().CurrentEpoch()
		if epoch.Equal(types.NewBlockHeight(0)) {
			actorCodeCid = types.BootstrapMinerActorCodeCid
		}

		initParams := []interface{}{vmctx.Message().Caller(), vmctx.Message().Caller(), pid, sectorSize}

		// create miner actor by messaging the init actor and sending it collateral
		ret, _, err := vmctx.LegacySend(address.InitAddress, initactor.ExecMethodID, vmctx.Message().ValueReceived(), []interface{}{actorCodeCid, initParams})
		if err != nil {
			return nil, err
		}

		addr, err := address.NewFromBytes(ret[0])
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not convert init.Exec return value to address")
		}

		// retrieve id to key miner
		actorIDAddr, err := retreiveActorID(vmctx, addr)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not retrieve actor id addrs after initializing actor")
		}

		ctx := context.Background()

		state.Miners, err = actor.SetKeyValue(ctx, vmctx.Runtime().LegacyStorage(), state.Miners, actorIDAddr.String(), true)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not set miner key value for lookup with CID: %s", state.Miners)
		}

		return addr, nil
	})
	if err != nil {
		return address.Undef, errors.CodeError(err), err
	}

	return ret.(address.Address), 0, nil
}

// retriveActorId uses init actor to map an actorAddress to an id address
func retreiveActorID(vmctx runtime.InvocationContext, actorAddr address.Address) (address.Address, error) {
	ret, _, err := vmctx.LegacySend(address.InitAddress, initactor.GetActorIDForAddressMethodID, types.ZeroAttoFIL, []interface{}{actorAddr})
	if err != nil {
		return address.Undef, err
	}

	actorIDVal, err := abi.Deserialize(ret[0], abi.Integer)
	if err != nil {
		return address.Undef, errors.FaultErrorWrap(err, "could not convert actor id to big.Int")
	}

	return address.NewIDAddress(actorIDVal.Val.(*big.Int).Uint64())
}

// UpdateStorage is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing, faults or willful removal). The delta is in number of bytes.
func (*impl) updateStorage(vmctx runtime.InvocationContext, delta *types.BytesAmount) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miner := vmctx.Message().Caller()
		ctx := context.Background()

		miners, err := actor.LoadLookup(ctx, vmctx.Runtime().LegacyStorage(), state.Miners)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with CID: %s", state.Miners)
		}

		err = miners.Find(ctx, miner.String(), nil)
		if err != nil {
			if err == hamt.ErrNotFound {
				return nil, Errors[ErrUnknownMiner]
			}
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with address: %s", miner)
		}

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(delta)

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

func (a *impl) getLateMiners(vmctx runtime.InvocationContext) (*map[string]uint64, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	var state State
	ctx := context.Background()

	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miners := map[string]uint64{}
		lu, err := actor.LoadLookup(ctx, vmctx.Runtime().LegacyStorage(), state.Miners)
		if err != nil {
			return &miners, err
		}

		err = lu.ForEachValue(ctx, nil, func(key string, _ interface{}) error {
			addr, err := address.NewFromString(key)
			if err != nil {
				return err
			}

			var poStState uint64
			poStState, err = a.getMinerPoStState(vmctx, addr)
			if err != nil {
				return err
			}

			if poStState == miner.PoStStateUnrecoverable {
				miners[addr.String()] = poStState
			}
			return nil
		})
		return &miners, err
	})

	if err != nil {
		return nil, errors.CodeError(err), err
	}

	res, ok := ret.(*map[string]uint64)
	if !ok {
		return res, 1, errors.NewFaultErrorf("expected []address.Address to be returned, but got %T instead", ret)
	}

	return res, 0, nil
}

// GetTotalStorage returns the total amount of proven storage in the system.
func (*impl) getTotalStorage(vmctx runtime.InvocationContext) (*types.BytesAmount, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		return state.TotalCommittedStorage, nil
	})
	if err != nil {
		return nil, errors.CodeError(err), err
	}

	amt, ok := ret.(*types.BytesAmount)
	if !ok {
		return nil, 1, fmt.Errorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return amt, 0, nil
}

// GetSectorSize returns the sector size of the block chain
func (*impl) getProofsMode(vmctx runtime.InvocationContext) (types.ProofsMode, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return 0, internal.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		return state.ProofsMode, nil
	})
	if err != nil {
		return 0, errors.CodeError(err), err
	}

	size, ok := ret.(types.ProofsMode)
	if !ok {
		return 0, 1, fmt.Errorf("expected types.ProofsMode to be returned, but got %T instead", ret)
	}

	return size, 0, nil
}

func (*impl) getMinerPoStState(vmctx runtime.InvocationContext, minerAddr address.Address) (uint64, error) {
	msgResult, _, err := vmctx.LegacySend(minerAddr, miner.GetPoStState, types.ZeroAttoFIL, nil)
	if err != nil {
		return 0, err
	}

	res, err := abi.Deserialize(msgResult[0], abi.Integer)
	if err != nil {
		return 0, err
	}
	resbi := res.Val.(*big.Int)
	return resbi.Uint64(), nil
}

// isSupportedSectorSize produces a boolean indicating whether or not the
// provided sector size is valid given the network's proofs mode.
func isSupportedSectorSize(mode types.ProofsMode, sectorSize *types.BytesAmount) bool {
	if mode == types.TestProofsMode {
		return sectorSize.Equal(types.OneKiBSectorSize)
	} else {
		return sectorSize.Equal(types.TwoHundredFiftySixMiBSectorSize)
	}
}
