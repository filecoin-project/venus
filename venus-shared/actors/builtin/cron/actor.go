// FETCHED FROM LOTUS: builtin/cron/actor.go.template

package cron

import (
	"fmt"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/filecoin-project/go-state-types/manifest"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/types"
	"github.com/ipfs/go-cid"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"

	builtin2 "github.com/filecoin-project/specs-actors/v2/actors/builtin"

	builtin3 "github.com/filecoin-project/specs-actors/v3/actors/builtin"

	builtin4 "github.com/filecoin-project/specs-actors/v4/actors/builtin"

	builtin5 "github.com/filecoin-project/specs-actors/v5/actors/builtin"

	builtin6 "github.com/filecoin-project/specs-actors/v6/actors/builtin"

	builtin7 "github.com/filecoin-project/specs-actors/v7/actors/builtin"

	builtin16 "github.com/filecoin-project/go-state-types/builtin"
)

func Load(store adt.Store, act *types.Actor) (State, error) {
	if name, av, ok := actors.GetActorMetaByCode(act.Code); ok {
		if name != manifest.CronKey {
			return nil, fmt.Errorf("actor code is not cron: %s", name)
		}

		switch av {

		case actorstypes.Version8:
			return load8(store, act.Head)

		case actorstypes.Version9:
			return load9(store, act.Head)

		case actorstypes.Version10:
			return load10(store, act.Head)

		case actorstypes.Version11:
			return load11(store, act.Head)

		case actorstypes.Version12:
			return load12(store, act.Head)

		case actorstypes.Version13:
			return load13(store, act.Head)

		case actorstypes.Version14:
			return load14(store, act.Head)

		case actorstypes.Version15:
			return load15(store, act.Head)

		case actorstypes.Version16:
			return load16(store, act.Head)

		}
	}

	switch act.Code {

	case builtin0.CronActorCodeID:
		return load0(store, act.Head)

	case builtin2.CronActorCodeID:
		return load2(store, act.Head)

	case builtin3.CronActorCodeID:
		return load3(store, act.Head)

	case builtin4.CronActorCodeID:
		return load4(store, act.Head)

	case builtin5.CronActorCodeID:
		return load5(store, act.Head)

	case builtin6.CronActorCodeID:
		return load6(store, act.Head)

	case builtin7.CronActorCodeID:
		return load7(store, act.Head)

	}

	return nil, fmt.Errorf("unknown actor code %s", act.Code)
}

func MakeState(store adt.Store, av actorstypes.Version) (State, error) {
	switch av {

	case actorstypes.Version0:
		return make0(store)

	case actorstypes.Version2:
		return make2(store)

	case actorstypes.Version3:
		return make3(store)

	case actorstypes.Version4:
		return make4(store)

	case actorstypes.Version5:
		return make5(store)

	case actorstypes.Version6:
		return make6(store)

	case actorstypes.Version7:
		return make7(store)

	case actorstypes.Version8:
		return make8(store)

	case actorstypes.Version9:
		return make9(store)

	case actorstypes.Version10:
		return make10(store)

	case actorstypes.Version11:
		return make11(store)

	case actorstypes.Version12:
		return make12(store)

	case actorstypes.Version13:
		return make13(store)

	case actorstypes.Version14:
		return make14(store)

	case actorstypes.Version15:
		return make15(store)

	case actorstypes.Version16:
		return make16(store)

	}
	return nil, fmt.Errorf("unknown actor version %d", av)
}

var (
	Address = builtin16.CronActorAddr
	Methods = builtin16.MethodsCron
)

type State interface {
	Code() cid.Cid
	ActorKey() string
	ActorVersion() actorstypes.Version

	GetState() interface{}
}

func AllCodes() []cid.Cid {
	return []cid.Cid{
		(&state0{}).Code(),
		(&state2{}).Code(),
		(&state3{}).Code(),
		(&state4{}).Code(),
		(&state5{}).Code(),
		(&state6{}).Code(),
		(&state7{}).Code(),
		(&state8{}).Code(),
		(&state9{}).Code(),
		(&state10{}).Code(),
		(&state11{}).Code(),
		(&state12{}).Code(),
		(&state13{}).Code(),
		(&state14{}).Code(),
		(&state15{}).Code(),
		(&state16{}).Code(),
	}
}
