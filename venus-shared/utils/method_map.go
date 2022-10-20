package utils

import (
	"reflect"

	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
	exported3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/exported"
	exported4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/exported"
	exported5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/exported"
	exported6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/exported"
	exported7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/exported"
	_actors "github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/ipfs/go-cid"
)

type MethodMeta struct {
	Name string

	Params reflect.Type
	Ret    reflect.Type
}

// In the v8 version, different networks will have different actors(venus-shared/builtin-actors/builtin_actors_gen.go).
// Pay attention to the network type when using.
// By default, the actors of the mainnet are loaded.
var MethodsMap = map[cid.Cid]map[abi.MethodNum]MethodMeta{}

type actorsWithVersion struct {
	av     actorstypes.Version
	actors []builtin.RegistryEntry
}

func init() {
	loadMethodsMap()
}

func ReloadMethodsMap() {
	MethodsMap = make(map[cid.Cid]map[abi.MethodNum]MethodMeta)
	loadMethodsMap()
}

func loadMethodsMap() {
	// TODO: combine with the runtime actor registry.
	var actors []actorsWithVersion

	actors = append(actors, actorsWithVersion{av: actorstypes.Version0, actors: builtin.MakeRegistryLegacy(exported0.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version2, actors: builtin.MakeRegistryLegacy(exported2.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version3, actors: builtin.MakeRegistryLegacy(exported3.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version4, actors: builtin.MakeRegistryLegacy(exported4.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version5, actors: builtin.MakeRegistryLegacy(exported5.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version6, actors: builtin.MakeRegistryLegacy(exported6.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version7, actors: builtin.MakeRegistryLegacy(exported7.BuiltinActors())})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version8, actors: builtin.MakeRegistry(actorstypes.Version8)})
	actors = append(actors, actorsWithVersion{av: actorstypes.Version9, actors: builtin.MakeRegistry(actorstypes.Version9)})

	for _, awv := range actors {
		for _, actor := range awv.actors {
			// necessary to make stuff work
			ac := actor.Code()
			var realCode cid.Cid
			if awv.av >= actorstypes.Version8 {
				name := _actors.CanonicalName(builtin.ActorNameByCode(ac))

				realCode, _ = _actors.GetActorCodeID(awv.av, name)
			}

			exports := actor.Exports()
			methods := make(map[abi.MethodNum]MethodMeta, len(exports))

			// Explicitly add send, it's special.
			methods[builtin.MethodSend] = MethodMeta{
				Name:   "Send",
				Params: reflect.TypeOf(new(abi.EmptyValue)),
				Ret:    reflect.TypeOf(new(abi.EmptyValue)),
			}

			// Iterate over exported methods. Some of these _may_ be nil and
			// must be skipped.
			for number, export := range exports {
				if export.Method == nil {
					continue
				}

				ev := reflect.ValueOf(export.Method)
				et := ev.Type()

				methodMeta := MethodMeta{
					Name: export.Name,
					Ret:  et.Out(0),
				}

				if awv.av <= actorstypes.Version7 {
					// methods exported from specs-actors have the runtime as the first param, so we want et.In(1)
					methodMeta.Params = et.In(1)
				} else {
					// methods exported from go-state-types do not, so we want et.In(0)
					methodMeta.Params = et.In(0)
				}

				methods[abi.MethodNum(number)] = methodMeta
			}

			MethodsMap[actor.Code()] = methods
			if realCode.Defined() {
				MethodsMap[realCode] = methods
			}
		}
	}
}
