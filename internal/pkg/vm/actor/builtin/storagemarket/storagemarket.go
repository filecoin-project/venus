package storagemarket

import (
	"context"
	"fmt"
	"math/big"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/enccid"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/initactor"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor/builtin/miner"
	vmaddr "github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/dispatch"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
	"github.com/filecoin-project/specs-actors/actors/abi"
	specsbig "github.com/filecoin-project/specs-actors/actors/abi/big"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/ipfs/go-hamt-ipld"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

func init() {
	encoding.RegisterIpldCborType(struct{}{})
}

// Actor implements the filecoin storage market. It is responsible
// for starting up new miners, and keeping track of the total storage power in the network.
type Actor struct{}

// State is the storage market's storage.
type State struct {
	Miners enccid.Cid

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
	AddBalance
)

// NewActor returns a new storage market actor.
func NewActor() *actor.Actor {
	return actor.NewActor(builtin.StorageMarketActorCodeID, abi.NewTokenAmount(0))
}

//
// ExecutableActor impl for Actor
//

var _ dispatch.Actor = (*Actor)(nil)

// Exports implements `dispatch.Actor`
func (a *Actor) Exports() []interface{} {
	return []interface{}{}
}

// InitializeState stores the actor's initial data structure.
func (*Actor) InitializeState(handle runtime.ActorStateHandle, proofsModeInterface interface{}) error {
	proofsMode, ok := proofsModeInterface.(types.ProofsMode)
	if !ok {
		return fmt.Errorf("storage market actor init parameter is not a proofs mode")
	}

	initStorage := &State{
		TotalCommittedStorage: types.NewBytesAmount(0),
		ProofsMode:            proofsMode,
	}
	handle.Create(&initStorage)

	return nil
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
	ErrUnknownMiner:          fmt.Errorf("unknown miner"),
	ErrUnsupportedSectorSize: fmt.Errorf("sector size is not supported"),
}

// CreateStorageMiner creates a new miner which will commit sectors of the
// given size. The miners collateral is set by the value in the message.
func (*impl) createStorageMiner(vmctx runtime.InvocationContext, sectorSize *types.BytesAmount, pid peer.ID) (address.Address, uint8, error) {
	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		if !isSupportedSectorSize(state.ProofsMode, sectorSize) {
			return nil, Errors[ErrUnsupportedSectorSize]
		}

		actorCodeCid := builtin.StorageMinerActorCodeID
		epoch := vmctx.Runtime().CurrentEpoch()
		if epoch == 0 {
			actorCodeCid = types.BootstrapMinerActorCodeCid
		}

		initParams := []interface{}{vmctx.Message().Caller(), vmctx.Message().Caller(), pid, sectorSize}

		// create miner actor by messaging the init actor and sending it collateral
		ret := vmctx.Send(vmaddr.InitAddress, initactor.ExecMethodID, vmctx.Message().ValueReceived(), []interface{}{actorCodeCid, initParams})
		addr := ret.(address.Address)

		// retrieve id to key miner
		actorIDAddr, err := retreiveActorID(vmctx, addr)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve actor id addrs after initializing actor")
		}

		ctx := context.Background()

		newMiners, err := actor.SetKeyValue(ctx, vmctx.Runtime().Storage(), state.Miners.Cid, string(actorIDAddr.Bytes()), true)
		state.Miners = enccid.NewCid(newMiners)
		if err != nil {
			return nil, fmt.Errorf("could not set miner key value for lookup with CID: %s", state.Miners)
		}

		return addr, nil
	})
	if err != nil {
		return address.Undef, 0, err
	}

	return ret.(address.Address), 0, nil
}

// retriveActorId uses init actor to map an actorAddress to an id address
func retreiveActorID(vmctx runtime.InvocationContext, actorAddr address.Address) (address.Address, error) {
	ret := vmctx.Send(vmaddr.InitAddress, initactor.GetActorIDForAddressMethodID, specsbig.Zero(), []interface{}{actorAddr})
	actorIDVal := ret.(big.Int)

	return address.NewIDAddress(actorIDVal.Uint64())
}

// UpdateStorage is called to reflect a change in the overall power of the network.
// This occurs either when a miner adds a new commitment, or when one is removed
// (via slashing, faults or willful removal). The delta is in number of bytes.
func (*impl) updateStorage(vmctx runtime.InvocationContext, delta *types.BytesAmount) (uint8, error) {
	var state State
	_, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		miner := vmctx.Message().Caller()
		ctx := context.Background()

		miners, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.Miners.Cid)
		if err != nil {
			return nil, fmt.Errorf("could not load lookup for miner with CID: %s", state.Miners)
		}

		err = miners.Find(ctx, string(miner.Bytes()), nil)
		if err != nil {
			if err == hamt.ErrNotFound {
				return nil, Errors[ErrUnknownMiner]
			}
			return nil, fmt.Errorf("could not load lookup for miner with address: %s", miner)
		}

		state.TotalCommittedStorage = state.TotalCommittedStorage.Add(delta)

		return nil, nil
	})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

func (a *impl) getLateMiners(vmctx runtime.InvocationContext) (*map[address.Address]uint64, uint8, error) {
	var state State
	ctx := context.Background()

	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		miners := map[address.Address]uint64{}
		lu, err := actor.LoadLookup(ctx, vmctx.Runtime().Storage(), state.Miners.Cid)
		if err != nil {
			return &miners, err
		}

		err = lu.ForEachValue(ctx, nil, func(key string, _ interface{}) error {
			addr, err := address.NewFromBytes([]byte(key))
			if err != nil {
				return err
			}

			var poStState uint64
			poStState, err = a.getMinerPoStState(vmctx, addr)
			if err != nil {
				return err
			}

			if poStState == miner.PoStStateUnrecoverable {
				miners[addr] = poStState
			}
			return nil
		})
		return &miners, err
	})

	if err != nil {
		return nil, 1, err
	}

	res, ok := ret.(*map[address.Address]uint64)
	if !ok {
		return res, 1, fmt.Errorf("expected []address.Address to be returned, but got %T instead", ret)
	}

	return res, 0, nil
}

// GetTotalStorage returns the total amount of proven storage in the system.
func (*impl) getTotalStorage(vmctx runtime.InvocationContext) (*types.BytesAmount, uint8, error) {
	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		return state.TotalCommittedStorage, nil
	})
	if err != nil {
		return nil, 1, err
	}

	amt, ok := ret.(*types.BytesAmount)
	if !ok {
		return nil, 1, fmt.Errorf("expected *big.Int to be returned, but got %T instead", ret)
	}

	return amt, 0, nil
}

// GetSectorSize returns the sector size of the block chain
func (*impl) getProofsMode(vmctx runtime.InvocationContext) (types.ProofsMode, uint8, error) {
	var state State
	ret, err := vmctx.State().Transaction(&state, func() (interface{}, error) {
		return state.ProofsMode, nil
	})
	if err != nil {
		return 0, 1, err
	}

	size, ok := ret.(types.ProofsMode)
	if !ok {
		return 0, 1, fmt.Errorf("expected types.ProofsMode to be returned, but got %T instead", ret)
	}

	return size, 0, nil
}

func (*impl) getMinerPoStState(vmctx runtime.InvocationContext, minerAddr address.Address) (uint64, error) {
	out := vmctx.Send(minerAddr, miner.GetPoStState, specsbig.Zero(), nil)
	res := out.(*big.Int)
	return res.Uint64(), nil
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
