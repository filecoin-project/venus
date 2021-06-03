package reward

import (
	"github.com/filecoin-project/go-state-types/abi"
	reward0 "github.com/filecoin-project/specs-actors/actors/builtin/reward"
	"github.com/filecoin-project/venus/pkg/specactors"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-state-types/cbor"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"

	builtin4 "github.com/filecoin-project/specs-actors/v4/actors/builtin"

	builtin5 "github.com/filecoin-project/specs-actors/v5/actors/builtin"

	"github.com/filecoin-project/venus/pkg/specactors/adt"
	"github.com/filecoin-project/venus/pkg/specactors/builtin"
	"github.com/filecoin-project/venus/pkg/types"
)

func init() {

	builtin.RegisterActorState(builtin0.RewardActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load0(store, root)
	})

	builtin.RegisterActorState(builtin2.RewardActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load2(store, root)
	})

	builtin.RegisterActorState(builtin3.RewardActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load3(store, root)
	})

	builtin.RegisterActorState(builtin4.RewardActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load4(store, root)
	})

	builtin.RegisterActorState(builtin5.RewardActorCodeID, func(store adt.Store, root cid.Cid) (cbor.Marshaler, error) {
		return load5(store, root)
	})
}

var (
	Address = builtin5.RewardActorAddr
	Methods = builtin5.MethodsReward
)

func Load(store adt.Store, act *types.Actor) (State, error) {
	switch act.Code {

	case builtin0.RewardActorCodeID:
		return load0(store, act.Head)

	case builtin2.RewardActorCodeID:
		return load2(store, act.Head)

	case builtin3.RewardActorCodeID:
		return load3(store, act.Head)

	case builtin4.RewardActorCodeID:
		return load4(store, act.Head)

	case builtin5.RewardActorCodeID:
		return load5(store, act.Head)

	}
	return nil, xerrors.Errorf("unknown actor code %s", act.Code)
}

func MakeState(store adt.Store, av specactors.Version, currRealizedPower abi.StoragePower) (State, error) {
	switch av {

	case specactors.Version0:
		return make0(store, currRealizedPower)

	case specactors.Version2:
		return make2(store, currRealizedPower)

	case specactors.Version3:
		return make3(store, currRealizedPower)

	case specactors.Version4:
		return make4(store, currRealizedPower)

	case specactors.Version5:
		return make5(store, currRealizedPower)

	}
	return nil, xerrors.Errorf("unknown actor version %d", av)
}

func GetActorCodeID(av specactors.Version) (cid.Cid, error) {
	switch av {

	case specactors.Version0:
		return builtin0.RewardActorCodeID, nil

	case specactors.Version2:
		return builtin2.RewardActorCodeID, nil

	case specactors.Version3:
		return builtin3.RewardActorCodeID, nil

	case specactors.Version4:
		return builtin4.RewardActorCodeID, nil

	case specactors.Version5:
		return builtin5.RewardActorCodeID, nil

	}

	return cid.Undef, xerrors.Errorf("unknown actor version %d", av)
}

type State interface {
	cbor.Marshaler

	ThisEpochBaselinePower() (abi.StoragePower, error)
	ThisEpochReward() (abi.StoragePower, error)
	ThisEpochRewardSmoothed() (builtin.FilterEstimate, error)

	EffectiveBaselinePower() (abi.StoragePower, error)
	EffectiveNetworkTime() (abi.ChainEpoch, error)

	TotalStoragePowerReward() (abi.TokenAmount, error)

	CumsumBaseline() (abi.StoragePower, error)
	CumsumRealized() (abi.StoragePower, error)

	InitialPledgeForPower(abi.StoragePower, abi.TokenAmount, *builtin.FilterEstimate, abi.TokenAmount) (abi.TokenAmount, error)
	PreCommitDepositForPower(builtin.FilterEstimate, abi.StoragePower) (abi.TokenAmount, error)
	GetState() interface{}
}

type AwardBlockRewardParams = reward0.AwardBlockRewardParams
