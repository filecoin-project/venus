package register

import (
	"bytes"
	"fmt"
	"sync"

	actorstypes "github.com/filecoin-project/go-state-types/actors"
	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
	exported3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/exported"
	exported4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/exported"
	exported5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/exported"
	exported6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/exported"
	exported7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/exported"
	"github.com/filecoin-project/venus/pkg/vm/dispatch"
	"github.com/filecoin-project/venus/pkg/vm/vmcontext"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
	"github.com/filecoin-project/venus/venus-shared/types"
)

// defaultActors is list of all actors that ship with Filecoin.
// They are indexed by their CID.
// Dragons: add the rest of the actors
var (
	DefaultActorBuilder = dispatch.NewBuilder()
	loadOnce            sync.Once
	defaultActors       dispatch.CodeLoader
)

func GetDefaultActros() *dispatch.CodeLoader {
	loadOnce.Do(func() {
		DefaultActorBuilder.AddMany(actorstypes.Version0, dispatch.ActorsVersionPredicate(actorstypes.Version0), builtin.MakeRegistryLegacy(exported0.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version2, dispatch.ActorsVersionPredicate(actorstypes.Version2), builtin.MakeRegistryLegacy(exported2.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version3, dispatch.ActorsVersionPredicate(actorstypes.Version3), builtin.MakeRegistryLegacy(exported3.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version4, dispatch.ActorsVersionPredicate(actorstypes.Version4), builtin.MakeRegistryLegacy(exported4.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version5, dispatch.ActorsVersionPredicate(actorstypes.Version5), builtin.MakeRegistryLegacy(exported5.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version6, dispatch.ActorsVersionPredicate(actorstypes.Version6), builtin.MakeRegistryLegacy(exported6.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version7, dispatch.ActorsVersionPredicate(actorstypes.Version7), builtin.MakeRegistryLegacy(exported7.BuiltinActors()))
		DefaultActorBuilder.AddMany(actorstypes.Version8, dispatch.ActorsVersionPredicate(actorstypes.Version8), builtin.MakeRegistry(actorstypes.Version8))
		DefaultActorBuilder.AddMany(actorstypes.Version9, dispatch.ActorsVersionPredicate(actorstypes.Version9), builtin.MakeRegistry(actorstypes.Version9))
		DefaultActorBuilder.AddMany(actorstypes.Version10, dispatch.ActorsVersionPredicate(actorstypes.Version10), builtin.MakeRegistry(actorstypes.Version10))
		DefaultActorBuilder.AddMany(actorstypes.Version11, dispatch.ActorsVersionPredicate(actorstypes.Version11), builtin.MakeRegistry(actorstypes.Version11))
		DefaultActorBuilder.AddMany(actorstypes.Version12, dispatch.ActorsVersionPredicate(actorstypes.Version12), builtin.MakeRegistry(actorstypes.Version12))
		DefaultActorBuilder.AddMany(actorstypes.Version13, dispatch.ActorsVersionPredicate(actorstypes.Version13), builtin.MakeRegistry(actorstypes.Version13))
		DefaultActorBuilder.AddMany(actorstypes.Version14, dispatch.ActorsVersionPredicate(actorstypes.Version14), builtin.MakeRegistry(actorstypes.Version14))
		DefaultActorBuilder.AddMany(actorstypes.Version15, dispatch.ActorsVersionPredicate(actorstypes.Version15), builtin.MakeRegistry(actorstypes.Version15))
		DefaultActorBuilder.AddMany(actorstypes.Version16, dispatch.ActorsVersionPredicate(actorstypes.Version16), builtin.MakeRegistry(actorstypes.Version16))
		defaultActors = DefaultActorBuilder.Build()
	})

	return &defaultActors
}

func DumpActorState(codeLoader *dispatch.CodeLoader, act *types.Actor, b []byte) (interface{}, error) {
	vmActor, err := codeLoader.GetVMActor(act.Code)
	if err != nil {
		return nil, fmt.Errorf("state type for actor %s not found", act.Code)
	}

	um := vmActor.State()
	if um == nil {
		if act.Head != vmcontext.EmptyObjectCid {
			return nil, fmt.Errorf("actor with code %s should only have empty object (%s) as its Head, instead has %s", act.Code, vmcontext.EmptyObjectCid, act.Head)
		}
		return nil, nil
	}
	if err := um.UnmarshalCBOR(bytes.NewReader(b)); err != nil {
		return nil, fmt.Errorf("unmarshaling actor state: %w", err)
	}

	return um, nil
}
