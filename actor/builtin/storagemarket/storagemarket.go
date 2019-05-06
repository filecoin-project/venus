package storagemarket

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-hamt-ipld"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/libp2p/go-libp2p-peer"

	"github.com/filecoin-project/go-filecoin/abi"
	"github.com/filecoin-project/go-filecoin/actor"
	"github.com/filecoin-project/go-filecoin/actor/builtin/miner"
	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/exec"
	"github.com/filecoin-project/go-filecoin/types"
	"github.com/filecoin-project/go-filecoin/vm/errors"
)

// MinimumPledge is the minimum amount of sectors a user can pledge.
var MinimumPledge = big.NewInt(10)

// MinimumCollateralPerSector is the minimum amount of collateral required per sector
var MinimumCollateralPerSector, _ = types.NewAttoFILFromFILString("0.001")

const (
	// ErrPledgeTooLow is the error code for a pledge under the MinimumPledge.
	ErrPledgeTooLow = 33
	// ErrUnknownMiner indicates a pledge under the MinimumPledge.
	ErrUnknownMiner = 34
	// ErrInsufficientCollateral indicates the collateral is too low.
	ErrInsufficientCollateral = 43
	// ErrUnsupportedSectorSize indicates that the sector size is incompatible with the proofs mode.
	ErrUnsupportedSectorSize = 44
)

// Errors map error codes to revert errors this actor may return.
var Errors = map[uint8]error{
	ErrPledgeTooLow:           errors.NewCodedRevertErrorf(ErrPledgeTooLow, "pledge must be at least %s sectors", MinimumPledge),
	ErrUnknownMiner:           errors.NewCodedRevertErrorf(ErrUnknownMiner, "unknown miner"),
	ErrInsufficientCollateral: errors.NewCodedRevertErrorf(ErrInsufficientCollateral, "collateral must be more than %s FIL per sector", MinimumCollateralPerSector),
	ErrUnsupportedSectorSize:  errors.NewCodedRevertErrorf(ErrUnsupportedSectorSize, "sector size is not supported"),
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

	// TotalCommitedStorage is the number of sectors that are currently committed
	// in the whole network.
	TotalCommittedStorage *big.Int

	ProofsMode types.ProofsMode
}

// NewActor returns a new storage market actor.
func NewActor() (*actor.Actor, error) {
	return actor.NewActor(types.StorageMarketActorCodeCid, types.NewZeroAttoFIL()), nil
}

// InitializeState stores the actor's initial data structure.
func (sma *Actor) InitializeState(storage exec.Storage, proofsModeInterface interface{}) error {
	proofsMode := proofsModeInterface.(types.ProofsMode)

	initStorage := &State{
		TotalCommittedStorage: big.NewInt(0),
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
	"createMiner": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer, abi.Bytes, abi.PeerID},
		Return: []abi.Type{abi.Address},
	},
	"updatePower": &exec.FunctionSignature{
		Params: []abi.Type{abi.Integer},
		Return: nil,
	},
	"getTotalStorage": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.Integer},
	},
	"getProofsMode": &exec.FunctionSignature{
		Params: []abi.Type{},
		Return: []abi.Type{abi.ProofsMode},
	},
}

// CreateMiner creates a new miner with the a pledge of the given amount of sectors. The
// miners collateral is set by the value in the message.
func (sma *Actor) CreateMiner(vmctx exec.VMContext, pledge *big.Int, publicKey []byte, pid peer.ID) (address.Address, uint8, error) {
	if err := vmctx.Charge(actor.DefaultGasCost); err != nil {
		return address.Undef, exec.ErrInsufficientGas, errors.RevertErrorWrap(err, "Insufficient gas")
	}

	var state State
	ret, err := actor.WithState(vmctx, &state, func() (interface{}, error) {
		// TODO: #2530 - Add a sector size parameter to the Actor#CreateMiner
		// method and accept the value from the CLI.
		sectorSize := types.OneKiBSectorSize
		if state.ProofsMode == types.LiveProofsMode {
			sectorSize = types.TwoHundredFiftySixMiBSectorSize
		}

		if !isSupportedSectorSize(state.ProofsMode, sectorSize) {
			return nil, Errors[ErrUnsupportedSectorSize]
		}

		if pledge.Cmp(MinimumPledge) < 0 {
			// TODO This should probably return a non-zero exit code instead of an error.
			return nil, Errors[ErrPledgeTooLow]
		}

		addr, err := vmctx.AddressForNewActor()
		if err != nil {
			return nil, errors.FaultErrorWrap(err, "could not get address for new actor")
		}

		if vmctx.Message().Value.LessThan(MinimumCollateral(pledge)) {
			return nil, Errors[ErrInsufficientCollateral]
		}

		minerInitializationParams := miner.NewState(vmctx.Message().From, publicKey, pledge, pid, vmctx.Message().Value, sectorSize)

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

// UpdatePower is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing or willful removal). The delta is in number of sectors.
func (sma *Actor) UpdatePower(vmctx exec.VMContext, delta *big.Int) (uint8, error) {
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

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(state.TotalCommittedStorage, delta)

		return nil, nil
	})
	if err != nil {
		return errors.CodeError(err), err
	}

	return 0, nil
}

// GetTotalStorage returns the total amount of proven storage in the system.
func (sma *Actor) GetTotalStorage(vmctx exec.VMContext) (*big.Int, uint8, error) {
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

	count, ok := ret.(*big.Int)
	if !ok {
		return nil, 1, fmt.Errorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return count, 0, nil
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

// MinimumCollateral returns the minimum required amount of collateral for a given pledge
func MinimumCollateral(sectors *big.Int) *types.AttoFIL {
	return MinimumCollateralPerSector.MulBigInt(sectors)
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
