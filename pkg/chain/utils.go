package chain

import (
	"context"
	"reflect"
	"runtime"
	"strings"

	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/rt"
	blockFormat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"

	exported0 "github.com/filecoin-project/specs-actors/actors/builtin/exported"
	exported2 "github.com/filecoin-project/specs-actors/v2/actors/builtin/exported"
	exported3 "github.com/filecoin-project/specs-actors/v3/actors/builtin/exported"
	exported4 "github.com/filecoin-project/specs-actors/v4/actors/builtin/exported"
	exported5 "github.com/filecoin-project/specs-actors/v5/actors/builtin/exported"
	exported6 "github.com/filecoin-project/specs-actors/v6/actors/builtin/exported"
	exported7 "github.com/filecoin-project/specs-actors/v7/actors/builtin/exported"
	exported8 "github.com/filecoin-project/specs-actors/v8/actors/builtin/exported"

	_actors "github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin"
)

type MethodMeta struct {
	Name string

	Params reflect.Type
	Ret    reflect.Type
}

var MethodsMap = map[cid.Cid]map[abi.MethodNum]MethodMeta{}

type actorsWithVersion struct {
	av     _actors.Version
	actors []rt.VMActor
}

func init() {
	// TODO: combine with the runtime actor registry.
	var actors []actorsWithVersion

	actors = append(actors, actorsWithVersion{av: _actors.Version0, actors: exported0.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version2, actors: exported2.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version3, actors: exported3.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version4, actors: exported4.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version5, actors: exported5.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version6, actors: exported6.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version7, actors: exported7.BuiltinActors()})
	actors = append(actors, actorsWithVersion{av: _actors.Version8, actors: exported8.BuiltinActors()})

	for _, awv := range actors {
		for _, actor := range awv.actors {
			// necessary to make stuff work
			ac := actor.Code()
			var realCode cid.Cid
			if awv.av >= _actors.Version8 {
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
				if export == nil {
					continue
				}

				ev := reflect.ValueOf(export)
				et := ev.Type()

				// Extract the method names using reflection. These
				// method names always match the field names in the
				// `builtin.Method*` structs (tested in the specs-actors
				// tests).
				fnName := runtime.FuncForPC(ev.Pointer()).Name()
				fnName = strings.TrimSuffix(fnName[strings.LastIndexByte(fnName, '.')+1:], "-fm")

				switch abi.MethodNum(number) {
				case builtin.MethodSend:
					panic("method 0 is reserved for Send")
				case builtin.MethodConstructor:
					if fnName != "Constructor" {
						panic("method 1 is reserved for Constructor")
					}
				}

				methods[abi.MethodNum(number)] = MethodMeta{
					Name:   fnName,
					Params: et.In(1),
					Ret:    et.Out(0),
				}
			}
			MethodsMap[actor.Code()] = methods
			if realCode.Defined() {
				MethodsMap[realCode] = methods
			}
		}
	}
}

type storable interface {
	ToStorageBlock() (blockFormat.Block, error)
}

func PutMessage(ctx context.Context, bs blockstoreutil.Blockstore, m storable) (cid.Cid, error) {
	b, err := m.ToStorageBlock()
	if err != nil {
		return cid.Undef, err
	}

	if err := bs.Put(ctx, b); err != nil {
		return cid.Undef, err
	}

	return b.Cid(), nil
}

// Reverse reverses the order of the slice `chain`.
func Reverse(chain []*types.TipSet) {
	// https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(chain)/2 - 1; i >= 0; i-- {
		opp := len(chain) - 1 - i
		chain[i], chain[opp] = chain[opp], chain[i]
	}
}
