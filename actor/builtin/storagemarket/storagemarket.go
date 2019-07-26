package storagemarket

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

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

func init() {
	cbor.RegisterCborType(State{})
	cbor.RegisterCborType(struct{}{})
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

// NewActor returns a new storage market actor.
func NewActor() *actor.Actor {
	return actor.NewActor(types.StorageMarketActorCodeCid, types.ZeroAttoFIL)
}

// InitializeState stores the actor's initial data structure.
func (sma *Actor) InitializeState(storage exec.Storage, proofsModeInterface interface{}) error {
	proofsMode := proofsModeInterface.(types.ProofsMode)

	initStorage := &State{
		TotalCommittedStorage: types.NewBytesAmount(0),
		ProofsMode:            proofsMode,
	}
	stateBytes, err := cbor.DumpObject(initStorage)
	if err != nil {
		return err
	}

	id, err := storage.Put(stateBytes)
	if err != nil {
		return err
	}

	return storage.Commit(id, cid.Undef)
}

var _ exec.ExecutableActor = (*Actor)(nil)

// Exports returns the actors exports.
func (sma *Actor) Exports() exec.Exports {
	return storageMarketExports
}

var storageMarketExports = exec.Exports{
	"createStorageMiner": &exec.FunctionSignature{
		Params: []abi.Type{abi.BytesAmount, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	"updateStorage": &exec.FunctionSignature{
		Params: []abi.Type{abi.BytesAmount},
		Return: nil,
	},
	"getTotalStorage": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.BytesAmount},
	},
	"getProofsMode": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.ProofsMode},
	},
	"getLateMiners": &exec.FunctionSignature{
		Params: nil,
		Return: []abi.Type{abi.MinerPoStStates},
	},
}

// CreateStorageMiner creates a new miner which will commit sectors of the
// given size. The miners collateral is set by the value in the message.
func (sma *Actor) CreateStorageMiner(vmctx exec.VMContext, sectorSize *types.BytesAmount, pid peer.ID) (address.Address, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		if !isSupportedSectorSize(state.ProofsMode, sectorSize) {
			return nil, Errors[ErrUnsupportedSectorSize]
		}

		addr, err := vmctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		minerInitializationParams := miner.NewState(vmctx.Message().From, vmctx.Message().From, pid, sectorSize)

		actorCodeCid := types.MinerActorCodeCid
		if vmctx.BlockHeight().Equal(types.NewBlockHeight(0)) {
			actorCodeCid = types.BootstrapMinerActorCodeCid
		}

		if err := vmctx.CreateNewActor(addr, actorCodeCid, minerInitializationParams); err != nil {
			return nil, err
		}

		_, _, err = vmctx.Send(addr, "", vmctx.Message().Value, nil)
		if err != nil {
			return nil, err
		}

		ctx := context.Background()

		state.Miners, err = actor.SetKeyValue(ctx, vmctx.Storage(), state.Miners, addr.String(), true)
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

// UpdateStorage is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing, faults or willful removal). The delta is in number of bytes.
func (sma *Actor) UpdateStorage(vmctx exec.VMContext, delta *types.BytesAmount) (uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	_, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miner := vmctx.Message().From
		ctx := context.Background()

		miners, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Miners)
		if err != nil {
			return nil, errors.FaultErrorWrapf(err, "could not load lookup for miner with CID: %s", state.Miners)
		}

		_, err = miners.Find(ctx, miner.String())
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

func (sma *Actor) GetLateMiners(vmctx exec.VMContext) (*map[string]uint64, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}
	var state State
	ctx := context.Background()

	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		miners := map[string]uint64{}
		lu, err := actor.LoadLookup(ctx, vmctx.Storage(), state.Miners)
		if err != nil {
			return &miners, err
		}

		vals, err := lu.Values(ctx)
		if err != nil {
			return &miners, err
		}

		for _, el := range vals {
			addr, err := address.NewFromString(el.Key)
			if err != nil {
				return &miners, err
			}

			var poStState uint64
			poStState, err = sma.getMinerPoStState(vmctx, addr)
			if err != nil {
				return &miners, err
			}

			if poStState > miner.PoStStateWithinProvingPeriod {
				miners[addr.String()] = poStState
			}
		}
		return &miners, nil
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
func (sma *Actor) GetTotalStorage(vmctx exec.VMContext) (*types.BytesAmount, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return nil, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
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
func (sma *Actor) GetProofsMode(vmctx exec.VMContext) (types.ProofsMode, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return 0, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
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

func (sma *Actor) getMinerPoStState(vmctx exec.VMContext, minerAddr address.Address) (uint64, error) {
	msgResult, _, err := vmctx.Send(minerAddr, "getPoStState", types.ZeroAttoFIL, nil)
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
