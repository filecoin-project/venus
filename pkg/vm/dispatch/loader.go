package dispatch

import (
	"github.com/filecoin-project/go-state-types/exitcode"
	rtt "github.com/filecoin-project/go-state-types/rt"
	rt5 "github.com/filecoin-project/specs-actors/v5/actors/runtime"
	"github.com/ipfs/go-cid"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/venus/pkg/types/specactors"
	vmr "github.com/filecoin-project/venus/pkg/vm/runtime"
)

// CodeLoader allows you to load an actor's code based on its id an epoch.
type CodeLoader struct {
	actors map[cid.Cid]ActorInfo
}

//ActorInfo vm contract actor
type ActorInfo struct {
	vmActor rtt.VMActor
	// TODO: consider making this a network version range?
	predicate ActorPredicate
}

// GetActorImpl returns executable code for an actor by code cid at a specific network version
func (cl CodeLoader) GetActorImpl(code cid.Cid, rt vmr.Runtime) (Dispatcher, *ExcuteError) {
	//todo version check
	actor, ok := cl.actors[code]
	if !ok {
		return nil, NewExcuteError(exitcode.SysErrorIllegalActor, "Actor code not found. code: %s", code)
	}
	if err := actor.predicate(rt, actor.vmActor); err != nil {
		return nil, NewExcuteError(exitcode.SysErrorIllegalActor, "unsupport actor. code: %s", code)
	}

	return &actorDispatcher{code: code, actor: actor.vmActor}, nil
}

// GetActorImpl returns executable code for an actor by code cid at a specific protocol version
func (cl CodeLoader) GetUnsafeActorImpl(code cid.Cid) (Dispatcher, error) {
	//todo version check
	actor, ok := cl.actors[code]
	if !ok {
		return nil, xerrors.Errorf("unable to get actorv for code %s", code)
	}
	return &actorDispatcher{code: code, actor: actor.vmActor}, nil
}

// CodeLoaderBuilder helps you build a CodeLoader.
type CodeLoaderBuilder struct {
	actors map[cid.Cid]ActorInfo
}

// NewBuilder creates a builder to generate a builtin.Actor data structure
func NewBuilder() *CodeLoaderBuilder {
	return &CodeLoaderBuilder{actors: map[cid.Cid]ActorInfo{}}
}

// Add lets you add an actor dispatch table for a given version.
func (b *CodeLoaderBuilder) Add(predict ActorPredicate, actor Actor) *CodeLoaderBuilder {
	if predict == nil {
		predict = func(vmr.Runtime, rtt.VMActor) error { return nil }
	}

	b.actors[actor.Code()] = ActorInfo{
		vmActor:   actor,
		predicate: predict,
	}
	return b
}

// Add lets you add an actor dispatch table for a given version.
func (b *CodeLoaderBuilder) AddMany(predict ActorPredicate, actors ...rt5.VMActor) *CodeLoaderBuilder {
	for _, actor := range actors {
		b.Add(predict, actor)
	}
	return b
}

// Build builds the code loader.
func (b *CodeLoaderBuilder) Build() CodeLoader {
	return CodeLoader{actors: b.actors}
}

// An ActorPredicate returns an error if the given actor is not valid for the given runtime environment (e.g., chain height, version, etc.).
type ActorPredicate func(vmr.Runtime, rtt.VMActor) error

//ActorsVersionPredicate  get actor predicate base on actor version and network version
func ActorsVersionPredicate(ver specactors.Version) ActorPredicate {
	return func(rt vmr.Runtime, v rtt.VMActor) error {
		nver, err := specactors.VersionForNetwork(rt.NtwkVersion())
		if err != nil {
			return xerrors.Errorf("version for network %w", err)
		}
		if nver != ver {
			return xerrors.Errorf("actor %s is a version %d actor; chain only supports actor version %d at height %d", v.Code(), ver, nver, rt.CurrentEpoch())
		}
		return nil
	}
}
