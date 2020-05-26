package vmcontext

import (
	specsruntime "github.com/filecoin-project/specs-actors/actors/runtime"
	"github.com/filecoin-project/specs-actors/actors/runtime/exitcode"
	"github.com/ipfs/go-cid"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/internal/runtime"
)

type actorStateHandle struct {
	ctx actorStateHandleContext
	// validations is a list of validations that the vm will execute after the actor code finishes.
	//
	// Any validation failure will result in the execution getting aborted.
	validations []validateFn
	// used_objs holds the pointers to objs that have been used with this handle and their expected state cid.
	usedObjs map[interface{}]cid.Cid
}

// validateFn returns True if it's valid.
type validateFn = func() bool

type actorStateHandleContext interface {
	AllowSideEffects(bool)
	Create(obj specsruntime.CBORMarshaler) cid.Cid
	Load(obj specsruntime.CBORUnmarshaler) cid.Cid
	Replace(expected cid.Cid, obj specsruntime.CBORMarshaler) cid.Cid
}

// NewActorStateHandle returns a new `ActorStateHandle`
//
// Note: just visible for testing.
func NewActorStateHandle(ctx actorStateHandleContext) specsruntime.StateHandle {
	aux := newActorStateHandle(ctx)
	return &aux
}

func newActorStateHandle(ctx actorStateHandleContext) actorStateHandle {
	return actorStateHandle{
		ctx:         ctx,
		validations: []validateFn{},
		usedObjs:    map[interface{}]cid.Cid{},
	}
}

var _ specsruntime.StateHandle = (*actorStateHandle)(nil)

func (h *actorStateHandle) Create(obj specsruntime.CBORMarshaler) {
	// Store the new state.
	c := h.ctx.Create(obj)
	// Store the expected CID of obj.
	h.usedObjs[obj] = c
}

// Readonly is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Readonly(obj specsruntime.CBORUnmarshaler) {
	// Load state to obj.
	c := h.ctx.Load(obj)
	// Track the state and expected CID used by the caller.
	h.usedObjs[obj] = c
}

// Transaction is the implementation of the ActorStateHandle interface.
func (h *actorStateHandle) Transaction(obj specsruntime.CBORer, f func() interface{}) interface{} {
	if obj == nil {
		runtime.Abortf(exitcode.SysErrorIllegalActor, "Must not pass nil to Transaction()")
	}

	// Load state to obj.
	prior := h.ctx.Load(obj)

	// Call user code allowing mutation but not side-effects
	h.ctx.AllowSideEffects(false)
	out := f()
	h.ctx.AllowSideEffects(true)

	// Store the new state
	newCid := h.ctx.Replace(prior, obj)

	// Record the expected state of obj
	h.usedObjs[obj] = newCid
	return out
}

// Validate validates that the state was mutated properly.
//
// This method is not part of the public API,
// it is expected to be called by the runtime after each actor method.
func (h *actorStateHandle) Validate(cidFn func(interface{}) cid.Cid) {
	for obj, head := range h.usedObjs {
		// verify the obj has not changed
		usedCid := cidFn(obj)
		if usedCid != head {
			runtime.Abortf(exitcode.SysErrorIllegalActor, "State mutated outside of Transaction() scope")
		}
	}
}
